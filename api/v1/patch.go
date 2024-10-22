package v1

import (
	"fmt"

	jsonpatch "github.com/evanphx/json-patch"
	"sigs.k8s.io/kustomize/api/hasher"
	kustresource "sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/yaml"
)

// Patch represent either a Strategic Merge Patch or a JSON patch and its targets.
type Patch struct {
	// Patch is the content of a patch.
	//+kubebuilder:validation:MinLength=1
	//+kubebuilder:validation:Required
	Patch string `json:"patch,omitempty" yaml:"patch,omitempty"`

	// Target points to the resources that the patch is applied to
	Target *Selector `json:"target,omitempty" yaml:"target,omitempty"`

	// Options is a list of options for the patch
	// +kubebuilder:validation:Optional
	Options map[string]bool `json:"options,omitempty" yaml:"options,omitempty"`
}

// Selector specifies a set of resources.
// Any resource that matches all of the conditions is included in this set.
type Selector struct {
	Group   string `json:"group,omitempty" yaml:"group,omitempty"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	Kind    string `json:"kind,omitempty" yaml:"kind,omitempty"`

	// Name of the resource.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Namespace the resource belongs to, if it can belong to a namespace.
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

	// AnnotationSelector is a string that follows the label selection expression
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#api
	// It matches against the resource annotations.
	AnnotationSelector string `json:"annotationSelector,omitempty" yaml:"annotationSelector,omitempty"`

	// LabelSelector is a string that follows the label selection expression
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#api
	// It matches against the resource labels.
	LabelSelector string `json:"labelSelector,omitempty" yaml:"labelSelector,omitempty"`
}

func (p *Patch) GetTarget() *Selector {
	if p.Target != nil {
		return p.Target
	}

	res, err := p.GetStrategicMergePatch()
	if err != nil {
		return nil
	}

	return &Selector{
		Name:      res.GetName(),
		Namespace: res.GetNamespace(),
		Group:     res.GetGvk().Group,
		Version:   res.GetGvk().Version,
		Kind:      res.GetKind(),
	}
}

func (p *Patch) GetStrategicMergePatch() (*kustresource.Resource, error) {
	factory := kustresource.NewFactory(&hasher.Hasher{})
	ress, err := factory.SliceFromBytes([]byte(p.Patch))
	if err != nil {
		return nil, err
	}

	if len(ress) != 1 {
		return nil, fmt.Errorf("expected strategic merge patch to contain exactly 1 resource, got %d", len(ress))
	}

	return ress[0], nil
}

// GetJsonPatch loads a JSON 6902 patch.
// Taken from sigs.k8s.io/kustomize/api@v0.12.1/internal/builtins/PatchTransformer.go
func (p *Patch) GetJsonPatch() (jsonpatch.Patch, error) {
	if p.Patch == "" {
		return nil, fmt.Errorf("empty json patch operations")
	}

	if p.Patch[0] != '[' {
		jsonOps, err := yaml.YAMLToJSON([]byte(p.Patch))
		if err != nil {
			return nil, err
		}
		p.Patch = string(jsonOps)
	}

	return jsonpatch.DecodePatch([]byte(p.Patch))
}
