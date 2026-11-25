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
	// Namespace is the Kubernetes namespace of the target pod.
	Namespace string
	// PodName is the name of the pod to attach the ephemeral container to.
	PodName string
	// ContainerName is the ephemeral container name (defaults upstream when empty).
	ContainerName string
	// Image is the image for the ephemeral container.
	Image string
	// ImagePullPolicy controls when the ephemeral container image is pulled (defaults to IfNotPresent).
	ImagePullPolicy corev1.PullPolicy
	// TargetProcess is the target container name to share a PID namespace with.
	TargetProcess string
}

// BuildEphemeralContainer constructs an EphemeralContainer spec from opts.
func BuildEphemeralContainer(opts EphemeralOptions) corev1.EphemeralContainer {
	ephemeralContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            opts.ContainerName,
			Image:           opts.Image,
			ImagePullPolicy: resolvePullPolicy(opts.ImagePullPolicy),
			Stdin:           true,
			TTY:             true,
		},
	}

	if opts.TargetProcess != "" {
		ephemeralContainer.TargetContainerName = opts.TargetProcess
	}

	return ephemeralContainer
}

// BuildDebugPod fetches the target Pod and appends the ephemeral container without updating the cluster.
func BuildDebugPod(ctx context.Context, client kubernetes.Interface, opts EphemeralOptions) (*corev1.Pod, error) {
	pod, err := client.CoreV1().Pods(opts.Namespace).Get(ctx, opts.PodName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, BuildEphemeralContainer(opts))

	return pod, nil
}

// AddEphemeralContainer adds an ephemeral container to an existing Pod.
func AddEphemeralContainer(ctx context.Context, client kubernetes.Interface, opts EphemeralOptions) error {
	pod, err := BuildDebugPod(ctx, client, opts)
	if err != nil {
		return err
	}

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
	hookMu.Lock()
	attachURLGetter := AttachURLGetter
	spdyCreator := SPDYExecutorCreator
	hookMu.Unlock()

	u, err := attachURLGetter(client, namespace, podName, containerName)
	if err != nil {
		return err
	}

	executor, err := spdyCreator(config, "POST", u)
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
