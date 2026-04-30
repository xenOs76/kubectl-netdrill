package k8s

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// EphemeralOptions defines options for adding an ephemeral container.
type EphemeralOptions struct {
	Namespace     string
	PodName       string
	ContainerName string
	Image         string
	TargetProcess string // Container name to share PID namespace with
}

// AddEphemeralContainer adds an ephemeral container to an existing Pod.
func AddEphemeralContainer(ctx context.Context, client kubernetes.Interface, opts EphemeralOptions) error {
	pod, err := client.CoreV1().Pods(opts.Namespace).Get(ctx, opts.PodName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	ephemeralContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            opts.ContainerName,
			Image:           opts.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Stdin:           true,
			TTY:             true,
		},
	}

	if opts.TargetProcess != "" {
		ephemeralContainer.TargetContainerName = opts.TargetProcess
	}

	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, ephemeralContainer)

	_, err = client.CoreV1().Pods(opts.Namespace).UpdateEphemeralContainers(
		ctx, opts.PodName, pod, metav1.UpdateOptions{})

	return err
}

// WaitForEphemeralContainerReady waits for the ephemeral container to be ready.
var WaitForEphemeralContainerReady = func(
	ctx context.Context,
	client kubernetes.Interface,
	namespace, podName, containerName string,
) error {
	watch, err := client.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", podName),
	})
	if err != nil {
		return err
	}
	defer watch.Stop()

	for {
		select {
		case event := <-watch.ResultChan():
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			if isRunning, err := checkEphemeralContainer(pod, containerName); err != nil {
				return err
			} else if isRunning {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// checkEphemeralContainer checks if the named ephemeral container is running in the pod.
func checkEphemeralContainer(pod *corev1.Pod, name string) (bool, error) {
	for _, status := range pod.Status.EphemeralContainerStatuses {
		if status.Name == name {
			if status.State.Running != nil {
				return true, nil
			}

			if status.State.Terminated != nil {
				return false, fmt.Errorf("ephemeral container terminated: %s", status.State.Terminated.Reason)
			}
		}
	}

	return false, nil
}

// AttachToEphemeralContainer attaches to the ephemeral container.
func AttachToEphemeralContainer(ctx context.Context, client kubernetes.Interface, config *rest.Config,
	namespace, podName, containerName string, tsq remotecommand.TerminalSizeQueue,
) error {
	u, err := AttachURLGetter(client, namespace, podName, containerName)
	if err != nil {
		return err
	}

	executor, err := SPDYExecutorCreator(config, "POST", u)
	if err != nil {
		return err
	}

	return executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             os.Stdin,
		Stdout:            os.Stdout,
		Stderr:            os.Stderr,
		Tty:               true,
		TerminalSizeQueue: tsq,
	})
}
