package controller

import "github.com/piraeusdatastore/piraeus-operator/pkg/controller/piraeusnodeset"

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, piraeusnodeset.Add)
}
