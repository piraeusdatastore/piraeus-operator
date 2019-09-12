/*
* A helpful library to interact with Linstor
* Copyright © 2018 LINBIT USA LCC
*
* This program is free software; you can redistribute it and/or modify
* it under the terms of the GNU General Public License as published by
* the Free Software Foundation; either version 2 of the License, or
* (at your option) any later version.
*
* This program is distributed in the hope that it will be useful,
* but WITHOUT ANY WARRANTY; without even the implied warranty of
* MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
* GNU General Public License for more details.
*
* You should have received a copy of the GNU General Public License
* along with this program; if not, see <http://www.gnu.org/licenses/>.
 */

package linstor

//go:generate ./linstor-common/genconsts.py golang apiconsts.go

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
)

// ResourceDeployment contains all the information needed to query and assign/deploy
// a resource.
type ResourceDeployment struct {
	ResourceDeploymentConfig
	autoPlaced    bool
	autoPlaceArgs []string
	log           *log.Entry
}

// ResourceDeploymentConfig is a configuration object for ResourceDeployment.
// If you're deploying a resource, AutoPlace is required. If you're
// assigning a resource to particular nodes, NodeList is required.
type ResourceDeploymentConfig struct {
	Name                string
	NodeList            []string
	ClientList          []string
	ReplicasOnSame      []string
	ReplicasOnDifferent []string
	DRSites             []string
	DRSiteKey           string
	AutoPlace           uint64
	DisklessOnRemaining bool
	DoNotPlaceWithRegex string
	SizeKiB             uint64
	StoragePool         string
	DisklessStoragePool string
	Encryption          bool
	MigrateOnAttach     bool
	Controllers         string
	LayerList           []string
	Annotations         map[string]string
	LogOut              io.Writer
	LogFmt              log.Formatter
}

// NewResourceDeployment creates a new ResourceDeployment object. This tolerates
// some pretty janky ResourceDeploymentConfigs here is the breakdown of how
// that is handled:
// If no Name is given, a UUID is generated and used.
// If no NodeList is given, assignment will be automatically placed.
// If no ClientList is given, no client assignments will be made.
// If there are duplicates within ClientList or NodeList, they will be removed.
// If there are duplicates between ClientList and NodeList, duplicates in the ClientList will be removed.
// If no AutoPlace Value is given AND there is no NodeList and no ClientList, it will default to 1.
// If no DisklessOnRemaining is given, no diskless assignments on non replica nodes will be made.
// If no DoNotPlaceWithRegex, ReplicasOnSame, or ReplicasOnDifferent are provided resource assignment will occur without them.
// If no SizeKiB is provided, it will be given a size of 4096kb.
// If no StoragePool is provided, the default storage pool will be used.
// If no DisklessStoragePool is provided, the default diskless storage pool will be used.
// If no Encryption is specified, none will be used.
// If no MigrateOnAttach is specified, none will be used.
// If no Controllers are specified, none will be used.
// If no LogOut is specified, ioutil.Discard will be used.
// TODO: Document DR stuff.
func NewResourceDeployment(c ResourceDeploymentConfig) ResourceDeployment {
	r := ResourceDeployment{ResourceDeploymentConfig: c}

	if r.Name == "" {
		r.Name = fmt.Sprintf("auto-%s", uuid.NewV4())
	}

	if len(r.NodeList) == 0 && len(r.ClientList) == 0 && r.AutoPlace == 0 {
		r.AutoPlace = 1
	}
	if r.AutoPlace > 0 {
		r.autoPlaced = true
	}

	r.NodeList = uniq(r.NodeList)
	r.ClientList = uniq(r.ClientList)
	r.ClientList = subtract(r.NodeList, r.ClientList)

	if r.SizeKiB == 0 {
		r.SizeKiB = 4096
	}

	if r.StoragePool == "" {
		r.StoragePool = "DfltStorPool"
	}

	if r.DisklessStoragePool == "" {
		r.DisklessStoragePool = "DfltDisklessStorPool"
	}

	if r.DisklessOnRemaining {
		r.autoPlaceArgs = append(r.autoPlaceArgs, "--diskless-on-remaining")
	}

	if r.DoNotPlaceWithRegex != "" {
		r.autoPlaceArgs = append(r.autoPlaceArgs, "--do-not-place-with-regex", r.DoNotPlaceWithRegex)
	}

	if len(r.ReplicasOnSame) != 0 {
		r.autoPlaceArgs = append(r.autoPlaceArgs, "--replicas-on-same")
		r.autoPlaceArgs = append(r.autoPlaceArgs, r.ReplicasOnSame...)
	}
	if len(r.ReplicasOnDifferent) != 0 {
		r.autoPlaceArgs = append(r.autoPlaceArgs, "--replicas-on-different")
		r.autoPlaceArgs = append(r.autoPlaceArgs, r.ReplicasOnDifferent...)
	}

	if r.LogOut == nil {
		r.LogOut = ioutil.Discard
	}

	if r.DRSiteKey == "" {
		r.DRSiteKey = "DR-site"
	}

	if r.LogFmt != nil {
		log.SetFormatter(r.LogFmt)
	}
	if r.LogOut == nil {
		r.LogOut = ioutil.Discard
	}
	log.SetOutput(r.LogOut)

	r.log = log.WithFields(log.Fields{
		"package":      "golinstor",
		"resourceName": r.Name,
	})

	return r
}

// uniq removes duplicates from a []string.
func uniq(strs []string) []string {
	seen := map[string]bool{}

	return unSeen(seen, strs)
}

// subtracts removes elements in s1 from s2.
func subtract(s1, s2 []string) []string {
	seen := map[string]bool{}
	for _, s := range s1 {
		seen[s] = true
	}

	return unSeen(seen, s2)
}

// unSeen returns a []string containing elements not present in seen.
func unSeen(seen map[string]bool, strs []string) []string {
	result := []string{}
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

type resList []struct {
	ResourceStates []struct {
		RequiresAdjust bool      `json:"requires_adjust"`
		RscName        string    `json:"rsc_name"`
		IsPrimary      bool      `json:"is_primary"`
		VlmStates      []volInfo `json:"vlm_states"`
		IsPresent      bool      `json:"is_present"`
		NodeName       string    `json:"node_name"`
	} `json:"resource_states"`
	Resources []resInfo `json:"resources"`
}
type resInfo struct {
	Vlms []struct {
		VlmNr        int    `json:"vlm_nr"`
		StorPoolName string `json:"stor_pool_name"`
		StorPoolUUID string `json:"stor_pool_uuid"`
		VlmMinorNr   int    `json:"vlm_minor_nr"`
		VlmUUID      string `json:"vlm_uuid"`
		VlmDfnUUID   string `json:"vlm_dfn_uuid"`
		MetaDisk     string `json:"meta_disk"`
		DevicePath   string `json:"device_path"`
		BackingDisk  string `json:"backing_disk"`
	} `json:"vlms"`
	NodeUUID string `json:"node_uuid"`
	UUID     string `json:"uuid"`
	NodeName string `json:"node_name"`
	Props    []struct {
		Value string `json:"value"`
		Key   string `json:"key"`
	} `json:"props"`
	RscDfnUUID string   `json:"rsc_dfn_uuid"`
	Name       string   `json:"name"`
	RscFlags   []string `json:"rsc_flags,omitempty"`
}

type volInfo struct {
	HasDisk       bool   `json:"has_disk"`
	CheckMetaData bool   `json:"check_meta_data"`
	HasMetaData   bool   `json:"has_meta_data"`
	IsPresent     bool   `json:"is_present"`
	DiskFailed    bool   `json:"disk_failed"`
	DiskState     string `json:"disk_state"`
	NetSize       int    `json:"net_size"`
	VlmMinorNr    *int   `json:"vlm_minor_nr"` // Allow nil checking.
	GrossSize     int    `json:"gross_size"`
	VlmNr         int    `json:"vlm_nr"`
}

type returnStatuses []struct {
	DetailsFormat string `json:"details_format"`
	MessageFormat string `json:"message_format"`
	CauseFormat   string `json:"cause_format,omitempty"`
	ObjRefs       []struct {
		Value string `json:"value"`
		Key   string `json:"key"`
	} `json:"obj_refs"`
	Variables []struct {
		Value string `json:"value"`
		Key   string `json:"key"`
	} `json:"variables"`
	RetCode uint64 `json:"ret_code"`
}

type resDefInfo []struct {
	ResDefList []ResDef `json:"rsc_dfns"`
}

type ResDef struct {
	VlmDfns []struct {
		VlmDfnUUID string `json:"vlm_dfn_uuid"`
		VlmMinor   int    `json:"vlm_minor"`
		VlmNr      int    `json:"vlm_nr"`
		VlmSize    int    `json:"vlm_size"`
	} `json:"vlm_dfns,omitempty"`
	RscDfnSecret string `json:"rsc_dfn_secret"`
	RscDfnUUID   string `json:"rsc_dfn_uuid"`
	RscName      string `json:"rsc_name"`
	RscDfnPort   int    `json:"rsc_dfn_port"`
	RscDfnProps  []struct {
		Value string `json:"value"`
		Key   string `json:"key"`
	} `json:"rsc_dfn_props,omitempty"`
}

type nodeInfo []struct {
	Nodes []struct {
		ConnectionStatus int    `json:"connection_status"`
		UUID             string `json:"uuid"`
		NetInterfaces    []struct {
			StltPort           int    `json:"stlt_port"`
			StltEncryptionType string `json:"stlt_encryption_type"`
			Address            string `json:"address"`
			UUID               string `json:"uuid"`
			Name               string `json:"name"`
		} `json:"net_interfaces"`
		Props []struct {
			Value string `json:"value"`
			Key   string `json:"key"`
		} `json:"props"`
		Type string `json:"type"`
		Name string `json:"name"`
	} `json:"nodes"`
}

type SnapshotInfo []struct {
	SnapshotDfns []Snapshot `json:"snapshot_dfns"`
}

type Snapshot struct {
	SnapshotDfnFlags []string `json:"snapshot_dfn_flags"`
	UUID             string   `json:"uuid"`
	RscName          string   `json:"rsc_name"`
	Nodes            []struct {
		NodeName string `json:"node_name"`
	} `json:"snapshots"`
	SnapshotName    string `json:"snapshot_name"`
	SnapshotVlmDfns []struct {
		VlmNr   int `json:"vlm_nr"`
		VlmSize int `json:"vlm_size"`
	} `json:"snapshot_vlm_dfns"`
	RscDfnUUID string `json:"rsc_dfn_uuid"`
}

func (s returnStatuses) validate() error {
	for _, message := range s {
		if !linstorSuccess(message.RetCode) {
			msg, err := json.Marshal(s)
			if err != nil {
				return err
			}
			return fmt.Errorf("error status from one or more linstor operations: %s", msg)
		}
	}
	return nil
}

func linstorSuccess(retcode uint64) bool {
	return (retcode & MaskError) == 0
}

// CreateAndAssign deploys the resource, created a new one if it doesn't exist.
func (r ResourceDeployment) CreateAndAssign() error {
	if err := r.Create(); err != nil {
		return err
	}
	return r.Assign()
}

func (r ResourceDeployment) prependOpts(args ...string) []string {
	a := []string{"-m"}
	if r.Controllers != "" {
		a = append(a, "--controllers", r.Controllers)
	}
	return append(a, args...)
}

func (r ResourceDeployment) traceCombinedOutput(name string, args ...string) ([]byte, error) {
	r.log.WithFields(log.Fields{
		"command": fmt.Sprintf("%s %s", name, strings.Join(args, " ")),
	}).Info("running external command")
	return exec.Command(name, args...).CombinedOutput()
}

// Only use this for things that return the normal returnStatuses json.
func (r ResourceDeployment) linstor(args ...string) error {
	out, err := r.traceCombinedOutput("linstor", r.prependOpts(args...)...)
	if err != nil {
		return fmt.Errorf("%v: %s", err, out)
	}

	if !json.Valid(out) {
		return fmt.Errorf("not a valid json input: %s", out)
	}
	s := returnStatuses{}
	if err := json.Unmarshal(out, &s); err != nil {
		return fmt.Errorf("couldn't Unmarshal %s :%v", out, err)
	}

	// Sometimes logs get separated, so it helps these to stay together,
	// even if it seems redundant.
	err = s.validate()
	if err != nil {
		return fmt.Errorf("failed to run command %q: %v", out, err)
	}
	return nil
}

func (r ResourceDeployment) ListResourceDefinitions() ([]ResDef, error) {
	list := resDefInfo{}
	out, err := r.traceCombinedOutput("linstor", r.prependOpts("resource-definition", "list")...)
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, out)
	}

	if !json.Valid(out) {
		return nil, fmt.Errorf("invalid json from 'linstor -m resource-definition list'")
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("couldn't Unmarshal '%s' :%v", out, err)
	}

	return list[0].ResDefList, nil
}

func (r ResourceDeployment) listResources() (resList, error) {
	list := resList{}
	out, err := r.traceCombinedOutput("linstor", r.prependOpts("resource", "list")...)
	if err != nil {
		return list, fmt.Errorf("%v: %s", err, out)
	}

	if !json.Valid(out) {
		return list, fmt.Errorf("invalid json from 'linstor -m resource list'")
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return list, fmt.Errorf("couldn't Unmarshal '%s' :%v", out, err)
	}

	return list, nil
}

func (r ResourceDeployment) listNodes() (nodeInfo, error) {
	list := nodeInfo{}
	out, err := r.traceCombinedOutput("linstor", r.prependOpts("node", "list")...)
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, out)
	}

	if !json.Valid(out) {
		return nil, fmt.Errorf("invalid json from 'linstor -m node list'")
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("couldn't Unmarshal '%s' :%v", out, err)
	}

	return list, nil
}

// Create reserves the resource name in Linstor.
func (r ResourceDeployment) Create() error {
	defPresent, volZeroPresent, err := r.checkDefined()
	if err != nil {
		return err
	}

	if !defPresent {
		args := []string{"resource-definition", "create", r.Name}
		if len(r.LayerList) != 0 {
			args = append(args, "--layer-list", strings.Join(r.LayerList, ","))
		}

		if err := r.linstor(args...); err != nil {
			return fmt.Errorf("unable to reserve resource name %s :%v", r.Name, err)
		}

		// Store annotations in the res def's aux props.
		for k, v := range r.Annotations {
			if err := r.SetAuxProp(k, v); err != nil {
				return err
			}
		}
	}

	if !volZeroPresent {

		args := []string{"volume-definition", "create", r.Name, fmt.Sprintf("%dkib", r.SizeKiB)}
		if r.Encryption {
			args = append(args, "--encrypt")
		}

		if err := r.linstor(args...); err != nil {
			return fmt.Errorf("unable to reserve resource name %s :%v", r.Name, err)
		}
	}

	return nil
}

// SnapshotCreate makes a snapshot of the ResourceDeployment on all nodes where
// it is deployed. If the snapshot already exsits, return it as is.
func (r ResourceDeployment) SnapshotCreate(name string) (*Snapshot, error) {
	snapshots, err := r.SnapshotList()
	if err != nil {
		return nil, fmt.Errorf("unable to create snapshot: %v", err)
	}

	// Return already existing snapshots.
	if snap := r.GetSnapByName(snapshots, name); snap != nil {
		return snap, nil
	}

	// Snapshot apparently not already present, try to create it.
	out, err := r.traceCombinedOutput("linstor", r.prependOpts("snapshot", "create", r.Name, name)...)
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, out)
	}

	// Try to retrive just created snapshot.
	snapshots, err = r.SnapshotList()
	if err != nil {
		return nil, fmt.Errorf("unable to create snapshot: %v", err)
	}
	snap := r.GetSnapByName(snapshots, name)
	if snap == nil {
		return nil, fmt.Errorf("snapshot not present after apparently successful creation")
	}

	return snap, nil
}

func (r ResourceDeployment) GetSnapByName(snapshots []Snapshot, name string) *Snapshot {
	for _, snap := range snapshots {
		if snap.SnapshotName == name && snap.RscName == r.Name {
			return &snap
		}
	}
	return nil
}

func (r ResourceDeployment) GetSnapByID(snapshots []Snapshot, id string) *Snapshot {
	for _, snap := range snapshots {
		if snap.UUID == id {
			return &snap
		}
	}
	return nil
}

func (r ResourceDeployment) NewResourceFromSnapshot(snapshotID string) error {
	defPresent, _, err := r.checkDefined()
	if err != nil {
		return err
	}

	if defPresent {
		return fmt.Errorf("resource definition %s already defined", r.Name)
	}
	if err := r.linstor("resource-definition", "create", r.Name); err != nil {
		return fmt.Errorf("unable to reserve resource name %s :%v", r.Name, err)
	}

	// Store annotations in the res def's aux props.
	for k, v := range r.Annotations {
		if err := r.SetAuxProp(k, v); err != nil {
			return err
		}
	}

	snapshots, err := r.SnapshotList()
	if err != nil {
		return fmt.Errorf("unable to create resource from snapshot: %v", err)
	}
	snap := r.GetSnapByID(snapshots, snapshotID)
	if snap == nil {
		return fmt.Errorf("unable to locate snapshot: %s", snapshotID)
	}

	if err := r.linstor(
		"snapshot", "volume-definition", "restore",
		"--from-resource", snap.RscName,
		"--from-snapshot", snap.SnapshotName,
		"--to-resource", r.Name); err != nil {
		return fmt.Errorf("unable to restore volume-definition:%v", err)
	}
	if err := r.linstor(
		"snapshot", "resource", "restore",
		"--from-resource", snap.RscName,
		"--from-snapshot", snap.SnapshotName,
		"--to-resource", r.Name); err != nil {
		return fmt.Errorf("unable to restore resource-definition:%v", err)
	}

	return nil
}

func (r ResourceDeployment) NewResourceFromResource(sourceRes ResourceDeployment) error {
	snap, err := sourceRes.SnapshotCreate(fmt.Sprintf("tmp-%s", uuid.NewV4()))
	if err != nil {
		return err
	}
	defer sourceRes.SnapshotDelete(snap.UUID)

	if err := r.NewResourceFromSnapshot(snap.UUID); err != nil {
		return err
	}

	return nil
}

// SnapshotDelete deletes a snapshot with the given ID.
func (r ResourceDeployment) SnapshotDelete(id string) error {
	snapshots, err := r.SnapshotList()
	if err != nil {
		return fmt.Errorf("unable to create snapshot: %v", err)
	}

	snap := r.GetSnapByID(snapshots, id)
	if snap == nil {
		return nil
	}

	out, err := r.traceCombinedOutput("linstor", r.prependOpts("snapshot", "delete", r.Name, snap.SnapshotName)...)
	if err != nil {
		return fmt.Errorf("%v: %s", err, out)
	}

	return nil
}

// SnapshotList returns a list of all snapshots known to LINSTOR.
func (r ResourceDeployment) SnapshotList() ([]Snapshot, error) {
	list := SnapshotInfo{}
	out, err := r.traceCombinedOutput("linstor", r.prependOpts("snapshot", "list")...)
	if err != nil {
		return []Snapshot{}, fmt.Errorf("%v: %s", err, out)
	}

	if !json.Valid(out) {
		return []Snapshot{}, fmt.Errorf("invalid json from 'linstor -m snapshot list'")
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return []Snapshot{}, fmt.Errorf("couldn't Unmarshal '%s' :%v", out, err)
	}

	return list[0].SnapshotDfns, nil
}

func (r ResourceDeployment) checkDefined() (bool, bool, error) {
	out, err := r.traceCombinedOutput("linstor", r.prependOpts("resource-definition", "list")...)
	if err != nil {
		return false, false, fmt.Errorf("%v: %s", err, out)
	}

	if !json.Valid(out) {
		return false, false, fmt.Errorf("not a valid json input: %s", out)
	}
	s := resDefInfo{}
	if err := json.Unmarshal(out, &s); err != nil {
		return false, false, fmt.Errorf("couldn't Unmarshal %s :%v", out, err)
	}

	var defPresent, volZeroPresent bool

	for _, def := range s[0].ResDefList {
		if def.RscName == r.Name {
			defPresent = true
			for _, vol := range def.VlmDfns {
				if vol.VlmNr == 0 {
					volZeroPresent = true
					break
				}
			}
			break
		}
	}

	return defPresent, volZeroPresent, nil
}

// Assign assigns a resource with diskfull storage to all nodes in its NodeList,
// then attaches the resource disklessly to all nodes in its ClientList.
func (r ResourceDeployment) Assign() error {

	if err := r.deployToList(r.NodeList, false); err != nil {
		return err
	}

	if r.autoPlaced {
		args := []string{"resource", "create", r.Name, "-s", r.StoragePool, "--auto-place", strconv.FormatUint(r.AutoPlace, 10)}
		args = append(args, r.autoPlaceArgs...)

		if err := r.linstor(args...); err != nil {
			return err
		}
	}

	if err := r.deployToList(r.ClientList, true); err != nil {
		return err
	}

	if len(r.ResourceDeploymentConfig.DRSites) > 0 {
		if err := r.enableProxy(); err != nil {
			return err
		}
	}
	return nil
}

// Attach assigns a resource on a single node disklessly or diskfully.
// If migrateOnAttach is true the migration will be be performed directly
// after assignment.
func (r ResourceDeployment) Attach(node string, asClient bool) error {
	if err := r.deployToList([]string{node}, asClient); err != nil {
		return err
	}

	if r.MigrateOnAttach && asClient {
		return r.migrateTo(node)
	}

	return nil
}

// Make node diskfull without changing the total number of replicas.
func (r ResourceDeployment) migrateTo(node string) error {
	diskfulNodes, err := r.DeployedNodes()
	if err != nil {
		return err
	}
	numDiskful := len(diskfulNodes)
	if numDiskful == 0 {
		return fmt.Errorf("resource %s has no healthy diskful assignments, unable to migrate", r.Name)
	}

	// We don't need to make this nodes diskfull it it happens to be diskful anyhow.
	if contains(diskfulNodes, node) {
		return nil
	}

	migrationSource := diskfulNodes[rand.Intn(numDiskful)]
	args := []string{"resource", "toggle-disk", node, r.Name, "--storage-pool", r.StoragePool, "--migrate-from", migrationSource}
	return r.linstor(args...)
}

func (r ResourceDeployment) deployToList(list []string, asClients bool) error {
	for _, node := range list {
		present, err := r.OnNode(node)
		if err != nil {
			return fmt.Errorf("unable to assign resource %s failed to check if it was already present on node %s: %v", r.Name, node, err)
		}

		args := []string{"resource", "create", node, r.Name, "-s"}

		if !present {
			if asClients {
				// If you're assigning to a diskless storage pool, you'll get warned that
				// your resource will be... diskless, unless you pass a flag that says
				// that it's... diskless
				args = append(args, r.DisklessStoragePool, "--diskless")
			} else {
				args = append(args, r.StoragePool)
			}
			if err = r.linstor(args...); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r ResourceDeployment) enableProxy() error {

	drNodes, err := r.proxyNodes()
	if err != nil {
		return err
	}

	// Figure out deployed nodes before assigning proxy resources.
	deployedNodes, err := r.DeployedNodes()
	if err != nil {
		return err
	}

	// Assign the resource to all proxy nodes.
	if err := r.deployToList(drNodes, false); err != nil {
		return err
	}

	for _, p := range drNodes {
		for _, d := range deployedNodes {
			if err := r.linstor("resource-connection", "drbd-options", "--protocol", "A", p, d, r.Name); err != nil {
				return nil
			}
			if err := r.linstor("drbd-proxy", "enable", p, d, r.Name); err != nil {
				return nil
			}
		}
	}
	return nil
}

func (r ResourceDeployment) proxyNodes() ([]string, error) {
	list, err := r.listNodes()
	if err != nil {
		return []string{}, nil
	}
	return doProxyNodes(list, r.DRSites, r.DRSiteKey)
}

func doProxyNodes(list nodeInfo, sites []string, key string) ([]string, error) {
	var proxySiteNodes []string
	auxPrefix := "Aux/"
	if !strings.HasPrefix(key, auxPrefix) {
		key = auxPrefix + key
	}

	for _, n := range list[0].Nodes {
		for _, p := range n.Props {
			if p.Key == key && contains(sites, p.Value) {
				proxySiteNodes = append(proxySiteNodes, n.Name)
			}
		}
	}

	return proxySiteNodes, nil
}

// DeployedNodes returns a list of nodes where the reource is deployed diskfully.
func (r ResourceDeployment) DeployedNodes() ([]string, error) {
	list, err := r.listResources()
	if err != nil {
		return []string{}, nil
	}
	return doDeployedNodes(list, r.Name)
}

func doDeployedNodes(l resList, resName string) ([]string, error) {
	var deployed []string
	for _, res := range l[0].Resources {
		if res.Name == resName && !contains(res.RscFlags, FlagDiskless) {
			deployed = append(deployed, res.NodeName)
		}
	}
	return deployed, nil
}

// Unassign unassigns a resource from a particular node.
func (r ResourceDeployment) Unassign(nodeName string) error {
	// If a reource doesn't exist, it's as unassigned as possible.
	defPresent, _, err := r.checkDefined()
	if err != nil {
		return fmt.Errorf("failed to unassign resource %s from node %s: %v", r.Name, nodeName, err)
	}
	if !defPresent {
		return nil
	}
	// If a resource isn't on the node, it's as unassigned as it can get.
	present, err := r.OnNode(nodeName)
	if err != nil {
		return fmt.Errorf("failed to unassign resource %s from node %s: %v", r.Name, nodeName, err)
	}
	if !present {
		return nil
	}

	if err := r.linstor("resource", "delete", nodeName, r.Name); err != nil {
		return fmt.Errorf("failed to unassign resource %s from node %s: %v", r.Name, nodeName, err)
	}
	return nil
}

// Delete removes a resource entirely from all nodes.
func (r ResourceDeployment) Delete() error {
	defPresent, _, err := r.checkDefined()
	if err != nil {
		return fmt.Errorf("failed to delete resource %s: %v", r.Name, err)
	}

	// If the resource definition doesn't exist, then the resource is as deleted
	// as we can possibly make it.
	if !defPresent {
		return nil
	}

	// If a resource has snapshots, then LINSTOR will refuse to delete it.
	// So we should need to clear those out.
	snaps, err := r.SnapshotList()
	if err != nil {
		return fmt.Errorf("failed to delete resource %s: %v", r.Name, err)
	}
	for _, snap := range snaps {
		if snap.RscName == r.Name {
			if err := r.SnapshotDelete(snap.UUID); err != nil {
				return fmt.Errorf("failed to delete resource %s: %v", r.Name, err)
			}
		}
	}

	if err := r.linstor("resource-definition", "delete", r.Name); err != nil {
		return fmt.Errorf("failed to delete resource %s: %v", r.Name, err)
	}
	return nil
}

// SetAuxProp adds an aux prop to the resource.
func (r ResourceDeployment) SetAuxProp(key, value string) error {
	if err := r.linstor("resource-definition", "set-property", "--aux", r.Name, key, value); err != nil {
		return fmt.Errorf("unable to set aux prop for resource %s :%v", r.Name, err)
	}

	return nil
}

// Exists checks to see if a resource is defined in LINSTOR.
func (r ResourceDeployment) Exists() (bool, error) {
	l, err := r.listResources()
	if err != nil {
		return false, err
	}

	// Inject real implementations here, test through the internal function.
	return doResExists(r.Name, l)
}

func doResExists(resourceName string, resources resList) (bool, error) {
	for _, r := range resources[0].Resources {
		if r.Name == resourceName {
			return true, nil
		}
	}

	return false, nil
}

//OnNode determines if a resource is present on a particular node.
func (r ResourceDeployment) OnNode(nodeName string) (bool, error) {
	l, err := r.listResources()
	if err != nil {
		return false, err
	}

	return doResOnNode(l, r.Name, nodeName), nil
}

func doResOnNode(list resList, resName, nodeName string) bool {
	for _, res := range list[0].Resources {
		if res.Name == resName && res.NodeName == nodeName {
			return true
		}
	}
	return false
}

// IsClient determines if resource is running as a client on nodeName.
func (r ResourceDeployment) IsClient(nodeName string) bool {
	l, err := r.listResources()
	if err != nil {
		return false
	}

	return r.doIsClient(l, nodeName)
}

func (r ResourceDeployment) doIsClient(list resList, nodeName string) bool {
	// Traverse all resources to find our resource on nodeName.
	for _, res := range list[0].Resources {
		if r.Name == res.Name && nodeName == res.NodeName && contains(res.RscFlags, FlagDiskless) {
			return true
		}
	}
	return false
}

func contains(data []string, candidate string) bool {
	for _, e := range data {
		if candidate == e {
			return true
		}
	}
	return false
}

// EnoughFreeSpace checks to see if there's enough free space to create a new resource.
func EnoughFreeSpace(requestedKiB, replicas string) error {
	return nil
}

// FSUtil handles creating a filesystem and mounting resources.
type FSUtil struct {
	*ResourceDeployment
	FSType    string
	FSOpts    string
	MountOpts string

	args []string
}

// Mount the FSUtil's resource on the path.
func (f FSUtil) Mount(source, target string) error {

	out, err := exec.Command("mkdir", "-p", target).CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to mount device, failed to make mount directory: %v: %s", err, out)
	}

	// If the path isn't mounted, then we're not mounted.
	mounted, err := f.isMounted(target)
	if err != nil {
		return fmt.Errorf("unable to unmount device: %q: %s", err, out)
	}
	if mounted {
		return nil
	}

	if f.MountOpts == "" {
		f.MountOpts = "defaults"
	}

	args := []string{"-o", f.MountOpts, source, target}

	out, err = f.traceCombinedOutput("mount", args...)
	if err != nil {
		return fmt.Errorf("unable to mount device: %v: %s", err, out)
	}

	return nil
}

// UnMount the FSUtil's resource from the path.
func (f FSUtil) UnMount(path string) error {
	// If the path isn't a directory, we're not mounted there.
	_, err := exec.Command("test", "-d", path).CombinedOutput()
	if err != nil {
		return nil
	}

	// If the path isn't mounted, then we're not mounted.
	mounted, err := f.isMounted(path)
	if err != nil {
		return fmt.Errorf("unable to unmount device: %v", err)
	}
	if !mounted {
		return nil
	}

	out, err := f.traceCombinedOutput("umount", path)
	if err != nil {
		return fmt.Errorf("unable to unmount device: %q: %s", err, out)
	}

	return nil
}

func (f FSUtil) SafeFormat(path string) error {

	// If if it's not a block device, don't touch it.
	_, err := exec.Command("test", "-b", path).CombinedOutput()
	if err != nil {
		return nil
	}
	deviceFS, err := checkFSType(path)
	if err != nil {
		return fmt.Errorf("unable to format filesystem for %q: %v", path, err)
	}

	// Device is formatted correctly already.
	if deviceFS == f.FSType {
		return nil
	}

	if deviceFS != "" && deviceFS != f.FSType {
		return fmt.Errorf("device %q already formatted with %q filesystem, refusing to overwrite with %q filesystem", path, deviceFS, f.FSType)
	}

	f.populateArgs()

	args := []string{"-t", f.FSType}
	args = append(args, f.args...)
	args = append(args, path)

	out, err := f.traceCombinedOutput("mkfs", args...)
	if err != nil {
		return fmt.Errorf("couldn't create %s filesystem on %s: %v: %q", f.FSType, path, err, out)
	}

	return nil
}

func (f *FSUtil) isMounted(path string) (bool, error) {
	out, err := exec.Command("findmnt", "-f", path).CombinedOutput()
	if err != nil {
		if string(out) != "" {
			return false, fmt.Errorf("%v: %s", err, out)
		}
		return false, nil
	}

	return true, nil
}

func (f *FSUtil) populateArgs() error {

	if f.FSOpts != "" {
		f.args = strings.Split(f.FSOpts, " ")
		return nil
	}

	return nil
}

func checkFSType(dev string) (string, error) {
	// If there's no filesystem, then we'll have a nonzero exit code, but no output
	// doCheckFSType handles this case.
	out, _ := exec.Command("blkid", "-o", "udev", dev).CombinedOutput()

	FSType, err := doCheckFSType(string(out))
	if err != nil {
		return "", err
	}
	return FSType, nil
}

// Parse the filesystem from the output of `blkid -o udev`
func doCheckFSType(s string) (string, error) {
	f := strings.Fields(s)

	// blkid returns an empty string if there's no filesystem and so do we.
	if len(f) == 0 {
		return "", nil
	}

	blockAttrs := make(map[string]string)
	for _, pair := range f {
		p := strings.Split(pair, "=")
		if len(p) < 2 {
			return "", fmt.Errorf("couldn't parse filesystem data from %s", s)
		}
		blockAttrs[p[0]] = p[1]
	}

	FSKey := "ID_FS_TYPE"
	fs, ok := blockAttrs[FSKey]
	if !ok {
		return "", fmt.Errorf("couldn't find %s in %s", FSKey, blockAttrs)
	}
	return fs, nil
}

// WaitForDevPath polls until the resourse path appears on the system.
func (r ResourceDeployment) WaitForDevPath(node string, maxRetries int) (string, error) {
	var path string
	var err error

	for i := 0; i < maxRetries; i++ {
		path, err = r.GetDevPath(node, true)
		if path != "" {
			return path, err
		}
		time.Sleep(time.Second * 2)
	}
	return path, err
}

// GetDevPath returns the path to the linstor volume on the given node.
// If stat is set to true, the device will be stat'd as a loose test to
// see if it is ready for IO.
func (r ResourceDeployment) GetDevPath(node string, stat bool) (string, error) {
	list, err := r.listResources()
	if err != nil {
		return "", err
	}

	devicePath, err := getDevPath(list, r.Name, node)
	if err != nil {
		return devicePath, err
	}

	if stat {
		if _, err := os.Lstat(devicePath); err != nil {
			return "", fmt.Errorf("Couldn't stat %s: %v", devicePath, err)
		}
	}

	return devicePath, nil
}

func getDevPath(list resList, resName, node string) (string, error) {
	// Traverse all the volume states to find volume 0 of our resource.
	// Assume volume 0 is the one we want.
	var devicePath string
	for _, res := range list[0].Resources {
		if resName == res.Name && node == res.NodeName {
			for _, v := range res.Vlms {
				if v.VlmNr == 0 {
					devicePath = v.DevicePath
					break
				}
			}
		}
	}

	if devicePath == "" {
		return devicePath, fmt.Errorf(
			"unable to find the device path of volume zero of resource %s on node %s in %+v", resName, node, list)
	}

	return devicePath, nil
}
