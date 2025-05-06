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

	return mux
}

type fakeLinstor struct {
	mu    sync.Mutex
	nodes []lapi.Node
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
	var node lapi.Node
	err := json.NewDecoder(r.Body).Decode(&node)
	if err != nil {
		return nil, fmt.Errorf("error decoding node: %v", err)
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

	f.nodes = slices.DeleteFunc(f.nodes, func(n lapi.Node) bool {
		return n.Name == node
	})

	return nil, nil
}
