// Package podexec runs a command inside a container of a running Pod and collects its output.
package podexec

import (
	"bytes"
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/streaming/pkg/httpstream"
)

// Executor runs a command in a container of a Pod and returns its standard output and standard error. The error
// is non-nil if the command could not be executed or exited with a non-zero status.
type Executor interface {
	Exec(ctx context.Context, namespace, pod, container string, command []string) (stdout, stderr string, err error)
}

// NewExecutor returns an Executor that uses the Kubernetes Pod exec subresource. It negotiates the connection
// using WebSocket with a fallback to SPDY, so it works across the range of supported Kubernetes versions.
func NewExecutor(config *rest.Config, clientset kubernetes.Interface) Executor {
	return &podExecutor{config: config, clientset: clientset}
}

type podExecutor struct {
	config    *rest.Config
	clientset kubernetes.Interface
}

func (e *podExecutor) Exec(ctx context.Context, namespace, pod, container string, command []string) (string, string, error) {
	req := e.clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Namespace(namespace).
		Name(pod).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	wsExec, err := remotecommand.NewWebSocketExecutor(e.config, "GET", req.URL().String())
	if err != nil {
		return "", "", fmt.Errorf("failed to create websocket executor: %w", err)
	}

	spdyExec, err := remotecommand.NewSPDYExecutor(e.config, "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("failed to create spdy executor: %w", err)
	}

	exec, err := remotecommand.NewFallbackExecutor(wsExec, spdyExec, func(err error) bool {
		return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to create executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: &stdout, Stderr: &stderr})
	return stdout.String(), stderr.String(), err
}
