package controller

import (
	"github.com/LINBIT/linstor-operator/pkg/controller/linstorsatelliteset"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, linstorsatelliteset.Add)
}
