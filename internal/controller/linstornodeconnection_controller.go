/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"sort"
	"time"

	linstor "github.com/LINBIT/golinstor"
	lapi "github.com/LINBIT/golinstor/client"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"gonum.org/v1/gonum/stat/combin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/conditions"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/linstorhelper"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/utils"
)

// LinstorNodeConnectionReconciler reconciles a LinstorNodeConnection object
type LinstorNodeConnectionReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Namespace         string
	LinstorClientOpts []lapi.Option
}

//+kubebuilder:rbac:groups=piraeus.io,resources=linstornodeconnections,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=piraeus.io,resources=linstornodeconnections/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=piraeus.io,resources=linstornodeconnections/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *LinstorNodeConnectionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	var allNodeConnections piraeusiov1.LinstorNodeConnectionList
	err := r.Client.List(ctx, &allNodeConnections)
	if err != nil {
		return ctrl.Result{}, err
	}

	var allLinstorSatellites piraeusiov1.LinstorSatelliteList
	err = r.Client.List(ctx, &allLinstorSatellites)
	if err != nil {
		return ctrl.Result{}, err
	}

	var allNodes corev1.NodeList
	err = r.Client.List(ctx, &allNodes)
	if err != nil {
		return ctrl.Result{}, err
	}

	conds := conditions.Conditions{}
	err = r.reconcileAll(ctx, allNodeConnections.Items, allLinstorSatellites.Items, allNodes.Items)
	if err != nil {
		conds.AddError(conditions.Configured, err)
	} else {
		conds.AddSuccess(conditions.Configured, "Configured")
	}

	var errs []error
	for i := range allNodeConnections.Items {
		conn := &allNodeConnections.Items[i]
		_, err := controllerutil.CreateOrPatch(ctx, r.Client, conn, func() error {
			for _, cond := range conds.ToConditions(conn.Generation) {
				meta.SetStatusCondition(&conn.Status.Conditions, cond)
			}

			return nil
		})
		if err != nil {
			errs = append(errs, err)
		}
	}

	result := ctrl.Result{
		RequeueAfter: 1 * time.Minute,
	}
	return result, utils.AnyError(errs...)
}

func (r *LinstorNodeConnectionReconciler) reconcileAll(ctx context.Context, conns []piraeusiov1.LinstorNodeConnection, satellites []piraeusiov1.LinstorSatellite, nodes []corev1.Node) error {
	sort.Slice(satellites, func(i, j int) bool {
		return satellites[i].Name < satellites[j].Name
	})

	nodeLabelMap := NodeLabelMap(nodes...)

	for _, view := range ClustersBySatellites(satellites) {
		desired := DesiredNodeConnections(conns, view.Satellites, nodeLabelMap)

		lc, err := linstorhelper.NewClientForCluster(
			ctx,
			r.Client,
			r.Namespace,
			view.ClusterRef.Name,
			view.ClusterRef.ClientSecretName,
			view.ClusterRef.ExternalController,
			append(
				slices.Clone(r.LinstorClientOpts),
				linstorhelper.Logr(log.FromContext(ctx)),
			)...,
		)
		if err != nil {
			return err
		}

		if lc == nil {
			return fmt.Errorf("controller unreachable")
		}

		actual, err := lc.Connections.GetNodeConnections(ctx, "", "")
		if err != nil {
			return err
		}

		for i := range actual {
			c := &actual[i]
			name := fmt.Sprintf("%s|%s", c.NodeA, c.NodeB)

			d := desired[name]
			delete(desired, name)

			mod := linstorhelper.MakePropertiesModification(c.Props, d.Props)
			if mod != nil {
				err := lc.Connections.SetNodeConnection(ctx, c.NodeA, c.NodeB, *mod)
				if err != nil {
					return err
				}
			}
		}

		for _, v := range desired {
			if !view.Satellites[v.NodeA] || !view.Satellites[v.NodeB] {
				// Skip configuration for satellites that are not configured: we will be notified of changes in any case
				continue
			}

			v.Props = linstorhelper.UpdateLastApplyProperty(v.Props)
			err := lc.Connections.SetNodeConnection(ctx, v.NodeA, v.NodeB, lapi.GenericPropsModify{OverrideProps: v.Props})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LinstorNodeConnectionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&piraeusiov1.LinstorNodeConnection{}).
		Watches(
			&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(r.allNodeConnectionsRequests),
			builder.WithPredicates(predicate.LabelChangedPredicate{}),
		).
		Watches(
			&piraeusiov1.LinstorSatellite{}, handler.EnqueueRequestsFromMapFunc(r.allNodeConnectionsRequests),
			builder.WithPredicates(predicate.LabelChangedPredicate{}),
		).
		Complete(r)
}

// We always reconcile all connections at once, so we only need to report one "fake" request here.
func (r *LinstorNodeConnectionReconciler) allNodeConnectionsRequests(_ context.Context, _ client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: "all"}}}
}

func NodeLabelMap(nodes ...corev1.Node) map[string]map[string]string {
	result := make(map[string]map[string]string)
	for i := range nodes {
		result[nodes[i].Name] = nodes[i].Labels
	}
	return result
}

type ClusterView struct {
	ClusterRef piraeusiov1.ClusterReference
	// Satellites maps the name of satellites to their ONLINE status.
	Satellites map[string]bool
}

// ClustersBySatellites collects all LinstorSatellite resource that are part of the same cluster.
//
// This should only ever return a map with a single entry, but it can support more.
func ClustersBySatellites(satellites []piraeusiov1.LinstorSatellite) map[string]*ClusterView {
	result := make(map[string]*ClusterView)
	for i := range satellites {
		sat := &satellites[i]

		view, ok := result[sat.Spec.ClusterRef.Name]
		if !ok {
			view = &ClusterView{
				ClusterRef: sat.Spec.ClusterRef,
				Satellites: map[string]bool{},
			}

			result[sat.Spec.ClusterRef.Name] = view
		}

		cond := meta.FindStatusCondition(sat.Status.Conditions, string(conditions.Available))
		view.Satellites[sat.Name] = cond != nil && cond.ObservedGeneration == sat.Generation && cond.Status == metav1.ConditionTrue
	}

	return result
}

func DesiredNodeConnections(conns []piraeusiov1.LinstorNodeConnection, satellites map[string]bool, nodeLabelMap map[string]map[string]string) map[string]lapi.Connection {
	if len(satellites) < 2 {
		// Obviously, nothing to reconcile in this case
		return nil
	}

	sortedSatellites := maps.Keys(satellites)
	sort.Strings(sortedSatellites)

	result := make(map[string]lapi.Connection)

	idx := make([]int, 2)
	comb := combin.NewCombinationGenerator(len(satellites), 2)
	for comb.Next() {
		comb.Combination(idx)
		nodeA := sortedSatellites[idx[0]]
		nodeB := sortedSatellites[idx[1]]

		c := lapi.Connection{
			NodeA: nodeA,
			NodeB: nodeB,
		}
		for i := range conns {
			if !NodeConnectionApplies(conns[i].Spec.Selector, nodeA, nodeB, nodeLabelMap) {
				continue
			}

			c.Props = MergeNodeConnection(c.Props, &conns[i], nodeA, nodeB)
		}

		if len(c.Props) > 0 {
			result[fmt.Sprintf("%s|%s", nodeA, nodeB)] = c
		}
	}

	return result
}

func MergeNodeConnection(props map[string]string, conn *piraeusiov1.LinstorNodeConnection, nodeA, nodeB string) map[string]string {
	p := utils.ResolveClusterProperties(nil, conn.Spec.Properties...)

	if props == nil {
		props = make(map[string]string)
	}

	for k, v := range p {
		props[k] = v
	}

	for i := range conn.Spec.Paths {
		path := &conn.Spec.Paths[i]
		props[fmt.Sprintf("%s/%s/%s", linstor.NamespcConnectionPaths, path.Name, nodeA)] = path.Interface
		props[fmt.Sprintf("%s/%s/%s", linstor.NamespcConnectionPaths, path.Name, nodeB)] = path.Interface
	}

	return props
}

func NodeConnectionApplies(selectors []piraeusiov1.SelectorTerm, nodeA, nodeB string, nodeLabelMap map[string]map[string]string) bool {
	if len(selectors) == 0 {
		return true
	}

	for _, term := range selectors {
		if evaluateTerm(term, nodeA, nodeB, nodeLabelMap) {
			return true
		}
	}

	return false
}

func evaluateTerm(term piraeusiov1.SelectorTerm, nodeA, nodeB string, nodeLabelMap map[string]map[string]string) bool {
	for _, expr := range term.MatchLabels {
		var b bool
		switch expr.Op {
		case piraeusiov1.MatchLabelSelectorOpExists:
			_, okA := nodeLabelMap[nodeA][expr.Key]
			_, okB := nodeLabelMap[nodeB][expr.Key]
			b = okA && okB
		case piraeusiov1.MatchLabelSelectorOpDoesNotExist:
			_, okA := nodeLabelMap[nodeA][expr.Key]
			_, okB := nodeLabelMap[nodeB][expr.Key]
			b = !okA && !okB
		case piraeusiov1.MatchLabelSelectorOpIn:
			valA := nodeLabelMap[nodeA][expr.Key]
			valB := nodeLabelMap[nodeB][expr.Key]
			b = slices.Contains(expr.Values, valA) && slices.Contains(expr.Values, valB)
		case piraeusiov1.MatchLabelSelectorOpNotIn:
			valA := nodeLabelMap[nodeA][expr.Key]
			valB := nodeLabelMap[nodeB][expr.Key]
			b = !slices.Contains(expr.Values, valA) && !slices.Contains(expr.Values, valB)
		case piraeusiov1.MatchLabelSelectorOpSame:
			b = nodeLabelMap[nodeA][expr.Key] == nodeLabelMap[nodeB][expr.Key]
		case piraeusiov1.MatchLabelSelectorOpNotSame:
			b = nodeLabelMap[nodeA][expr.Key] != nodeLabelMap[nodeB][expr.Key]
		}

		if !b {
			return false
		}
	}

	return true
}
