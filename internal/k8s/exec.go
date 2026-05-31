package k8s

import (
	"bytes"
	"context"
	"fmt"
	"net/url"

	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// ExecResult holds captured output from a non-interactive exec.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// ExecURLGetter builds the REST URL for pod exec.
var ExecURLGetter = func(
	client kubernetes.Interface,
	namespace, podName, containerName string,
	command []string,
) (*url.URL, error) {
	req := client.CoreV1().RESTClient().Post().
		Namespace(namespace).
		Resource("pods").
		Name(podName).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   command,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, scheme.ParameterCodec)

	return req.URL(), nil
}

// ExecCommand runs a non-interactive command in a pod container and captures output.
func ExecCommand(
	ctx context.Context,
	client kubernetes.Interface,
	config *rest.Config,
	namespace, podName, containerName string,
	command []string,
	maxBytes int64,
) (ExecResult, error) {
	if containerName == "" {
		containerName = netdrill.ContainerNetdrill
	}

	hookMu.Lock()
	execURLGetter := ExecURLGetter
	spdyCreator := SPDYExecutorCreator
	hookMu.Unlock()

	u, err := execURLGetter(client, namespace, podName, containerName, command)
	if err != nil {
		return ExecResult{}, err
	}

	executor, err := spdyCreator(config, "POST", u)
	if err != nil {
		return ExecResult{}, fmt.Errorf("creating executor: %w", err)
	}

	var stdoutBuf, stderrBuf bytes.Buffer

	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdoutBuf,
		Stderr: &stderrBuf,
		Tty:    false,
	})

	result := ExecResult{
		Stdout:   truncateBytes(stdoutBuf.String(), maxBytes),
		Stderr:   truncateBytes(stderrBuf.String(), maxBytes),
		ExitCode: 0,
	}

	if err != nil {
		if exitErr, ok := err.(interface{ ExitStatus() int }); ok {
			result.ExitCode = exitErr.ExitStatus()
		}

		return result, fmt.Errorf("exec stream: %w", err)
	}

	return result, nil
}

func truncateBytes(s string, byteLimit int64) string {
	if byteLimit <= 0 || int64(len(s)) <= byteLimit {
		return s
	}

	return s[:byteLimit]
}
