package linstorcontroller

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/sirupsen/logrus"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"

	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1"
	"github.com/piraeusdatastore/piraeus-operator/pkg/k8s/spec"
)

const (
	// BackupAnnotationPreviousVersion marks the annotation that stores the upgraded-from cluster version.
	BackupAnnotationPreviousVersion = spec.APIGroup + "/backup-previous-version"
	// BackupAnnotationUpdateVersion marks the annotations that stores the upgraded-to cluster version.
	BackupAnnotationUpdateVersion = spec.APIGroup + "/backup-update-version"
	// BackupLabel is used for all backup secrets
	BackupLabel = spec.APIGroup + "/linstor-backup"
)

// reconcileLinstorControllerDatabaseBackup ensures a backup of all LINSTOR database resources exists when the image is updated.
func (r *ReconcileLinstorController) reconcileLinstorControllerDatabaseBackup(ctx context.Context, controllerResource *piraeusv1.LinstorController) error {
	log := logrus.WithFields(logrus.Fields{
		"name":      controllerResource.Name,
		"namespace": controllerResource.Namespace,
		"spec":      fmt.Sprintf("%+v", controllerResource.Spec),
	})

	log.Info("reconciling LINSTOR Controller Database database")

	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to create rest config from in-cluster-config")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client from rest config")
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create dynamic kubernetes client from rest config")
	}

	// 1. Check if image version has changed
	previousVersion, err := getPreviousDeployment(ctx, clientset, controllerResource)
	if err != nil {
		return err
	}

	if previousVersion == controllerResource.Spec.ControllerImage {
		log.WithField("version", previousVersion).Info("image up to date, no backup necessary")

		return nil
	}

	// 2. Check if we have CRDs to back up
	crds, err := getLinstorCRDs(ctx, dynClient)
	if err != nil {
		return err
	}

	if len(crds) == 0 {
		log.Info("no resources to back up")

		return nil
	}

	// 3. Ensure backup exists
	err = createBackup(ctx, clientset, dynClient, controllerResource, previousVersion, crds)
	if err != nil {
		return err
	}

	return nil
}

func createBackup(ctx context.Context, clientset kubernetes.Interface, dynClient dynamic.Interface, controllerResource *piraeusv1.LinstorController, previousVersion string, crds []*unstructured.Unstructured) error {
	meta := getBackupMetadata(controllerResource, previousVersion)

	log.WithField("backup", meta.Name).Info("check for existing backup")

	_, err := clientset.CoreV1().Secrets(meta.Namespace).Get(ctx, meta.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check for existing backup: %w", err)
	}

	if err == nil {
		log.Info("backup already exists")

		return nil
	}

	filepath := "/run/linstor-backups/" + meta.Name + ".tar.gz"
	_ = os.MkdirAll("/run/linstor-backups", os.FileMode(0o755)) // nolint:gomnd // File permissions don't seem too magic to me

	_, err = os.Stat(filepath)
	if err == nil {
		log.Info("backup already exists in filesystem")

		return &manualDownloadRequiredError{secretName: meta.Name, namespace: meta.Namespace, filepath: filepath}
	}

	log.Debug("ensure LINSTOR Controller is offline while taking a resource snapshot")

	err = stopDeployment(ctx, clientset, controllerResource)
	if err != nil {
		return err
	}

	log.Info("collecting LINSTOR Controller database resources")

	backupContent, err := collectLinstorDatabase(ctx, dynClient, crds)
	if err != nil {
		return err
	}

	log.WithField("path", filepath).Info("persist LINSTOR backup to container fs location")

	err = ioutil.WriteFile(filepath, backupContent.Bytes(), os.FileMode(0o644)) // nolint:gomnd // File permissions don't seem too magic to me
	if err != nil {
		return fmt.Errorf("error writing LINSTOR backup to container fs location: %w", err)
	}

	yes := true
	backup := &corev1.Secret{
		ObjectMeta: *meta,
		Immutable:  &yes,
		Data: map[string][]byte{
			"backup.tar.gz": backupContent.Bytes(),
		},
		Type: corev1.SecretType(spec.APIGroup + "/linstor-backup"),
	}

	_, err = clientset.CoreV1().Secrets(meta.Namespace).Create(ctx, backup, metav1.CreateOptions{})

	if errors.IsRequestEntityTooLargeError(err) {
		return &manualDownloadRequiredError{secretName: meta.Name, namespace: meta.Namespace, filepath: filepath}
	}

	if err != nil {
		return fmt.Errorf("failed to create backup secret: %w", err)
	}

	return nil
}

type manualDownloadRequiredError struct {
	secretName string
	namespace  string
	filepath   string
}

func (e *manualDownloadRequiredError) Error() string {
	return fmt.Sprintf("failed to store LINSTOR database backup in Kubernetes API because it is too large\n"+
		"The backup has been written to '%s' in the operator pod. "+
		"Please manually copy it to a safe location:\n"+
		"  kubectl cp <pod>:/%s .\n"+
		"Then create an empty secret to indicate it is safe to continue:\n"+
		"  kubectl create secret generic -n %s %s", e.filepath, e.filepath, e.namespace, e.secretName)
}

// collectLinstorDatabase fetches all LINSTOR internal resources, including the defining CRDs.
func collectLinstorDatabase(ctx context.Context, dynClient dynamic.Interface, crds []*unstructured.Unstructured) (*bytes.Buffer, error) {
	buffer := &bytes.Buffer{}

	compressionWriter, err := gzip.NewWriterLevel(buffer, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("failed to create archive writer: %w", err)
	}

	defer compressionWriter.Close()

	archiveWriter := tar.NewWriter(compressionWriter)

	defer archiveWriter.Close()

	crdBuffer := &bytes.Buffer{}

	for _, linstorCrd := range crds {
		serializedCrd, err := ToCleanedK8sResourceYAML(linstorCrd)
		if err != nil {
			return nil, err
		}

		_, _ = crdBuffer.WriteString("---\n")
		_, _ = crdBuffer.Write(serializedCrd)

		serializedResources, err := getResourcesForCrd(ctx, dynClient, linstorCrd)
		if err != nil {
			return nil, err
		}

		err = addToArchive(archiveWriter, fmt.Sprintf("%s.yaml", linstorCrd.GetName()), serializedResources)
		if err != nil {
			return nil, err
		}
	}

	err = addToArchive(archiveWriter, "crds.yaml", crdBuffer.Bytes())
	if err != nil {
		return nil, err
	}

	return buffer, nil
}

// getLinstorCRDs returns all LINSTOR internal CRDs.
func getLinstorCRDs(ctx context.Context, dynClient dynamic.Interface) ([]*unstructured.Unstructured, error) {
	res := schema.GroupVersionResource{Group: "apiextensions.k8s.io", Resource: "customresourcedefinitions", Version: "v1"}

	crds, err := dynClient.Resource(res).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list CRDs: %w", err)
	}

	linstorCrds := make([]*unstructured.Unstructured, 0, len(crds.Items))

	for i := range crds.Items {
		crd := &crds.Items[i]

		group := getString(crd.Object, "spec", "group")
		if group != "internal.linstor.linbit.com" {
			continue
		}

		linstorCrds = append(linstorCrds, crd)
	}

	return linstorCrds, nil
}

// getResourcesForCrd returns a serialized form of all the resources of a specific type.
func getResourcesForCrd(ctx context.Context, dynClient dynamic.Interface, crd *unstructured.Unstructured) ([]byte, error) {
	versions := getList(crd.Object, "spec", "versions")
	if len(versions) == 0 {
		log.WithField("crd", crd.GetName()).Info("crd has no version, skipping")

		return nil, nil
	}

	group := getString(crd.Object, "spec", "group")
	resource := getString(crd.Object, "spec", "names", "plural")
	version := getString(versions[0], "name")

	res := schema.GroupVersionResource{Group: group, Resource: resource, Version: version}

	resources, err := dynClient.Resource(res).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to load resources for CRD %s: %w", res, err)
	}

	resourceBuffer := bytes.Buffer{}

	for j := range resources.Items {
		resourceYaml, err := ToCleanedK8sResourceYAML(&resources.Items[j])
		if err != nil {
			return nil, err
		}

		resourceBuffer.WriteString("---\n")
		resourceBuffer.Write(resourceYaml)
	}

	return resourceBuffer.Bytes(), nil
}

// getBackupMetadata creates kubernetes metadata for this backup based on the previous version and the version we
// upgrade to. This should uniquely identify every upgrade operation.
func getBackupMetadata(controllerResource *piraeusv1.LinstorController, previousVersion string) *metav1.ObjectMeta {
	id := previousVersion + "-" + controllerResource.Spec.ControllerImage
	sum := sha256.Sum256([]byte(id))
	digest := hex.EncodeToString(sum[:])

	backupName := fmt.Sprintf("linstor-backup-%s", digest)[:63]

	return &metav1.ObjectMeta{
		Name:      backupName,
		Namespace: controllerResource.Namespace,
		Labels: map[string]string{
			BackupLabel: "",
		},
		Annotations: map[string]string{
			BackupAnnotationPreviousVersion: previousVersion,
			BackupAnnotationUpdateVersion:   controllerResource.Spec.ControllerImage,
		},
	}
}

type Resource interface {
	metav1.Object
	metav1.Type
}

// ToCleanedK8sResourceYAML returns a stripped down version of the kubernetes object in YAML format.
//
// * It only contains human-controlled metadata (name, namespace, labels, annotations).
// * It does not contain a status section.
func ToCleanedK8sResourceYAML(obj Resource) ([]byte, error) {
	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert object to unstructured object: %w", err)
	}

	// No need to store the status
	delete(raw, "status")

	metadata := map[string]interface{}{
		"name": obj.GetName(),
	}

	if obj.GetNamespace() != "" {
		metadata["namespace"] = obj.GetNamespace()
	}

	if len(obj.GetLabels()) != 0 {
		metadata["labels"] = obj.GetLabels()
	}

	if len(obj.GetAnnotations()) != 0 {
		metadata["annotations"] = obj.GetAnnotations()
	}

	raw["metadata"] = metadata

	buf, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to convert unstructured object to json: %w", err)
	}

	return buf, nil
}

// getPreviousDeployment returns the name of the image currently deployed in the cluster.
func getPreviousDeployment(ctx context.Context, clientset kubernetes.Interface, controllerResource *piraeusv1.LinstorController) (string, error) {
	log := logrus.WithFields(logrus.Fields{
		"name":      controllerResource.Name,
		"namespace": controllerResource.Namespace,
	})

	meta := getObjectMeta(controllerResource, "%s-controller")

	log.WithField("meta", meta).Debug("fetching existing deployment")

	deployment, err := clientset.AppsV1().Deployments(meta.Namespace).Get(ctx, meta.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return "", fmt.Errorf("failed to check on existing deployment: %w", err)
	}

	if deployment == nil {
		log.Debug("no deployment found, previous deployment unknown")

		return "unknown", nil
	}

	log.Debug("got deployment")

	containers := deployment.Spec.Template.Spec.Containers

	for i := range containers {
		if containers[i].Name == "linstor-controller" {
			return containers[i].Image, nil
		}
	}

	return "", nil
}

// stopDeployment scale the deployment to 0 and waits for all pods to terminate.
func stopDeployment(ctx context.Context, clientset kubernetes.Interface, controllerResource *piraeusv1.LinstorController) error {
	meta := getObjectMeta(controllerResource, "%s-controller")

	_, err := clientset.AppsV1().Deployments(meta.Namespace).UpdateScale(ctx, meta.Name, &autoscalingv1.Scale{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: meta.Namespace,
		},
		Spec: autoscalingv1.ScaleSpec{Replicas: 0},
	}, metav1.UpdateOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// No deployment, nothing to stop
			return nil
		}

		return fmt.Errorf("failed to update controller scale: %w", err)
	}

	log.Debug("wait for pods to terminate")

	pods, err := clientset.CoreV1().Pods(meta.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: meta.Labels}),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	podsWatch, err := clientset.CoreV1().Pods(meta.Namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector:   metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: meta.Labels}),
		ResourceVersion: pods.ResourceVersion,
	})
	if err != nil {
		return fmt.Errorf("failed to set up watch for pods: %w", err)
	}

	defer podsWatch.Stop()

	count := len(pods.Items)

	for {
		log.WithField("count", count).Debug("watch remaining pods")

		if count == 0 {
			log.Debug("all pods terminated")

			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("error while waiting for pods to terminate: %w", ctx.Err())
		case ev := <-podsWatch.ResultChan():
			if ev.Type == watch.Deleted {
				count--
			} else if ev.Type == watch.Added {
				count++
			}
		}
	}
}

func addToArchive(archive *tar.Writer, name string, content []byte) error {
	hdr := &tar.Header{
		Name: name,
		Mode: 0o644, // nolint:gomnd // File permissions don't seem too magic to me
		Size: int64(len(content)),
	}

	err := archive.WriteHeader(hdr)
	if err != nil {
		return fmt.Errorf("failed to write archive header: %w", err)
	}

	_, err = archive.Write(content)
	if err != nil {
		return fmt.Errorf("failed to write to archive: %w", err)
	}

	return nil
}

func getString(obj interface{}, keys ...string) string {
	result, ok := unstructuredGet(obj, keys...).(string)
	if !ok {
		return ""
	}

	return result
}

func getList(obj interface{}, keys ...string) []interface{} {
	result, ok := unstructuredGet(obj, keys...).([]interface{})
	if !ok {
		return nil
	}

	return result
}

func unstructuredGet(obj interface{}, keys ...string) interface{} {
	for _, key := range keys[:] {
		m, ok := obj.(map[string]interface{})
		if !ok {
			return nil
		}

		obj = m[key]
	}

	return obj
}
