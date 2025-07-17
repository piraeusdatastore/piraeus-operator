package clusterapi

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	clusterapiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Client struct {
	client client.Client
}

func NewClientForConfig(cfg *rest.Config) (*Client, error) {
	scheme := runtime.NewScheme()
	err := clusterapiv1beta1.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("could not construct scheme: %w", err)
	}

	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("could not construct client: %w", err)
	}

	return &Client{
		client: cl,
	}, nil
}

func NewClient(cl client.Client) *Client {
	return &Client{client: cl}
}
