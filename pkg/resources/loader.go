package resources

import (
	"embed"
	"io/fs"

	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"
)

type Kustomizer struct {
	fsys       filesys.FileSystem
	kustomizer *krusty.Kustomizer
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
