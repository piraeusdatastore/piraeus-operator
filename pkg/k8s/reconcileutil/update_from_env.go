package reconcileutil

import (
	"context"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type EnvSpec struct {
	Env    string
	Target *string
}

func UpdateFromEnv(ctx context.Context, client client.Client, obj client.Object, specs ...EnvSpec) error {
	changed := false

	for i := range specs {
		if *specs[i].Target != "" {
			continue
		}

		val, ok := os.LookupEnv(specs[i].Env)
		if !ok {
			continue
		}

		changed = true
		*specs[i].Target = val
	}

	if !changed {
		return nil
	}

	return client.Update(ctx, obj)
}
