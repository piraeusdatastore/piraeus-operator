package reconcileutil

import (
	"context"
	"os"

	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type EnvSpec struct {
	Env    string
	Target *string
}

func UpdateFromEnv(ctx context.Context, client client.Client, obj runtime.Object, specs ...EnvSpec) error {
	changed := false

	for i := range specs {
		val, ok := os.LookupEnv(specs[i].Env)
		if ok {
			changed = true
			*specs[i].Target = val
		}
	}

	if !changed {
		return nil
	}

	return client.Update(ctx, obj)
}
