package resources

import (
	"embed"
	"io/fs"

	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/yaml"
)

type Kustomizer struct {
	fsys       filesys.FileSystem
	kustomizer *krusty.Kustomizer
}

func init() {
	// We need to inform kustomize that our own CRD are cluster scoped, i.e. they don't have a namespace.
	// Otherwise, we would have namespaces after kustomization on object that do not support it. This would then
	// potentially trip up our pruning logic. So we add a simple schema here, which then gets used internally by
	// kustomize to determine that our resources are cluster scoped.
	_ = openapi.AddSchema([]byte(`
{
  "definitions": {},
  "paths": {
    "/apis/piraeus.io/v1/linstorclusters": {
      "get": {
        "x-kubernetes-action": "get",
        "x-kubernetes-group-version-kind": {
          "group": "piraeus.io",
          "kind": "LinstorCluster",
          "version": "v1"
        }
      }
    },
    "/apis/piraeus.io/v1/linstorsatelliteconfigurations": {
      "get": {
        "x-kubernetes-action": "get",
        "x-kubernetes-group-version-kind": {
          "group": "piraeus.io",
          "kind": "LinstorSatelliteConfiguration",
          "version": "v1"
        }
      }
    },
    "/apis/piraeus.io/v1/linstorsatellites": {
      "get": {
        "x-kubernetes-action": "get",
        "x-kubernetes-group-version-kind": {
          "group": "piraeus.io",
          "kind": "LinstorSatellite",
          "version": "v1"
        }
      }
    }
  }
}
`))
}

func NewKustomizer(base *embed.FS, options *krusty.Options) (*Kustomizer, error) {
	fsys := filesys.MakeFsInMemory()

	err := fs.WalkDir(base, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			err := fsys.Mkdir(path)
			if err != nil {
				return err
			}
		} else {
			content, err := base.ReadFile(path)
			if err != nil {
				return err
			}
			err = fsys.WriteFile(path, content)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &Kustomizer{
		fsys:       fsys,
		kustomizer: krusty.MakeKustomizer(options),
	}, nil
}

func (k *Kustomizer) Kustomize(kustomization *types.Kustomization) (resmap.ResMap, error) {
	rawK, err := yaml.Marshal(kustomization)
	if err != nil {
		return nil, err
	}

	err = k.fsys.WriteFile("/kustomization.yaml", rawK)
	if err != nil {
		return nil, err
	}

	return k.kustomizer.Run(k.fsys, "/")
}
