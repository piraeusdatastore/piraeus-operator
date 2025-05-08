package fakelinstor

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	linstor "github.com/LINBIT/golinstor"
	lapi "github.com/LINBIT/golinstor/client"
	"golang.org/x/exp/slices"

	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
)

func New() http.Handler {
	f := &fakeLinstor{}

	mux := http.NewServeMux()
	mux.Handle("GET /v1/controller/version", WrapHandler(f.getVersion))
	mux.Handle("POST /v1/nodes", WrapHandler(f.createNode))
	mux.Handle("GET /v1/nodes/{node}", WrapHandler(f.getNode))
	mux.Handle("DELETE /v1/nodes/{node}", WrapHandler(f.deleteNode))
	mux.Handle("PUT /v1/nodes/{node}/evacuate", WrapHandler(f.evacuateNode))
	mux.Handle("POST /v1/resource-definitions", WrapHandler(f.createResourceDefinition))
	mux.Handle("DELETE /v1/resource-definitions/{rd}", WrapHandler(f.deleteResourceDefinition))
	mux.Handle("POST /v1/resource-definitions/{rd}/resources/{node}", WrapHandler(f.createResource))
	mux.Handle("GET /v1/view/resources", WrapHandler(f.viewResources))

	return mux
}

type fakeLinstor struct {
	mu                  sync.Mutex
	nodes               []lapi.Node
	resources           []lapi.ResourceWithVolumes
	resourceDefinitions []lapi.ResourceDefinition
}

func WrapHandler(f func(r *http.Request) (any, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := f(r)
		if err != nil {
			if errors.Is(err, lapi.NotFoundError) {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(lapi.ApiCallError{{RetCode: 1, Message: err.Error()}})
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(lapi.ApiCallError{{RetCode: 1, Message: err.Error()}})
			}
		} else {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(data)
		}
	})
}

func (f *fakeLinstor) getVersion(r *http.Request) (any, error) {
	return lapi.ControllerVersion{
		Version:        vars.Version,
		RestApiVersion: "fake-rest-api",
		GitHash:        "fake-git-hash",
		BuildTime:      "fake-build-time",
	}, nil
}

func (f *fakeLinstor) createNode(r *http.Request) (any, error) {
	defer r.Body.Close() //nolint:errcheck

	var node lapi.Node
	err := json.NewDecoder(r.Body).Decode(&node)
	if err != nil {
		return nil, fmt.Errorf("error decoding node: %w", err)
	}

	if len(node.NetInterfaces) > 0 && node.Type == linstor.ValNodeTypeStlt {
		node.ConnectionStatus = "ONLINE"
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.nodes {
		if f.nodes[i].Name == node.Name {
			return nil, fmt.Errorf("node '%s' already exists", node.Name)
		}
	}
	f.nodes = append(f.nodes, node)

	return nil, nil
}

func (f *fakeLinstor) getNode(r *http.Request) (any, error) {
	node := r.PathValue("node")

	f.mu.Lock()
	defer f.mu.Unlock()

	for i := range f.nodes {
		if f.nodes[i].Name == node {
			return f.nodes[i], nil
		}
	}

	return nil, lapi.NotFoundError
}

func (f *fakeLinstor) deleteNode(r *http.Request) (any, error) {
	node := r.PathValue("node")

	f.mu.Lock()
	defer f.mu.Unlock()

	for i := range f.resources {
		if f.resources[i].NodeName == node {
			return nil, fmt.Errorf("cannot delete node '%s', it still contains resource '%s'", node, f.resources[i].Name)
		}
	}

	f.nodes = slices.DeleteFunc(f.nodes, func(n lapi.Node) bool {
		return n.Name == node
	})

	return nil, nil
}

func (f *fakeLinstor) createResourceDefinition(r *http.Request) (any, error) {
	defer r.Body.Close() //nolint:errcheck

	var rd lapi.ResourceDefinitionCreate
	err := json.NewDecoder(r.Body).Decode(&rd)
	if err != nil {
		return nil, fmt.Errorf("error decoding resource definition: %w", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if slices.ContainsFunc(f.resourceDefinitions, func(exist lapi.ResourceDefinition) bool {
		return exist.Name == rd.ResourceDefinition.Name
	}) {
		return nil, fmt.Errorf("resource definition '%s' already exists", rd.ResourceDefinition.Name)
	}

	f.resourceDefinitions = append(f.resourceDefinitions, rd.ResourceDefinition)
	return nil, nil
}

func (f *fakeLinstor) createResource(r *http.Request) (any, error) {
	defer r.Body.Close() //nolint:errcheck

	var rc lapi.ResourceCreate
	err := json.NewDecoder(r.Body).Decode(&rc)
	if err != nil {
		return nil, fmt.Errorf("error decoding resource create: %w", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	rd := r.PathValue("rd")
	node := r.PathValue("node")
	rc.Resource.Name = rd
	rc.Resource.NodeName = node

	if !slices.ContainsFunc(f.resourceDefinitions, func(exist lapi.ResourceDefinition) bool {
		return exist.Name == rd
	}) {
		return nil, lapi.NotFoundError
	}

	if !slices.ContainsFunc(f.nodes, func(exist lapi.Node) bool {
		return exist.Name == node
	}) {
		return nil, lapi.NotFoundError
	}

	if !slices.ContainsFunc(f.resources, func(exist lapi.ResourceWithVolumes) bool {
		return exist.Name == rd && exist.NodeName == node
	}) {
		f.resources = append(f.resources, lapi.ResourceWithVolumes{
			Resource: rc.Resource,
		})
	}

	return nil, nil
}

func (f *fakeLinstor) deleteResourceDefinition(r *http.Request) (any, error) {
	rd := r.PathValue("rd")

	f.mu.Lock()
	defer f.mu.Unlock()

	f.resources = slices.DeleteFunc(f.resources, func(exist lapi.ResourceWithVolumes) bool {
		return exist.Name == rd
	})
	f.resourceDefinitions = slices.DeleteFunc(f.resourceDefinitions, func(exist lapi.ResourceDefinition) bool {
		return exist.Name == rd
	})

	return nil, nil
}

func (f *fakeLinstor) evacuateNode(r *http.Request) (any, error) {
	node := r.PathValue("node")

	f.mu.Lock()
	defer f.mu.Unlock()

	for i := range f.nodes {
		if f.nodes[i].Name == node {
			if !slices.Contains(f.nodes[i].Flags, linstor.FlagEvacuate) {
				f.nodes[i].Flags = append(f.nodes[i].Flags, linstor.FlagEvacuate)
			}
			return nil, nil
		}
	}

	return nil, lapi.NotFoundError
}

func (f *fakeLinstor) viewResources(r *http.Request) (any, error) {
	r.URL.Query()

	resources := slices.Clone(f.resources)
	if len(r.URL.Query()["nodes"]) > 0 {
		resources = slices.DeleteFunc(resources, func(exist lapi.ResourceWithVolumes) bool {
			return !slices.Contains(r.URL.Query()["nodes"], exist.NodeName)
		})
	}

	return resources, nil
}
