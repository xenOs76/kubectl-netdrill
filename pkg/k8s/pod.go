package k8s

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// PodOptions defines options for creating a troubleshooting Pod.
type PodOptions struct {
	Namespace    string
	PodName      string
	Image        string
	HostNetwork  bool
	Command      []string
	NodeSelector map[string]string
}

// CreatePod creates a new Pod with the specified options.
func CreatePod(ctx context.Context, client *kubernetes.Clientset, opts PodOptions) (*corev1.Pod, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.PodName,
			Namespace: opts.Namespace,
			Labels: map[string]string{
				"app": "kubectl-netdrill",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "netdrill",
					Image:           opts.Image,
					ImagePullPolicy: corev1.PullAlways,
					Stdin:           true,
					TTY:             true,
					Command:         opts.Command,
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
			HostNetwork:   opts.HostNetwork,
			NodeSelector:  opts.NodeSelector,
		},
	}

	return client.CoreV1().Pods(opts.Namespace).Create(ctx, pod, metav1.CreateOptions{})
}

// WaitForPodReady waits until the Pod is in Running state.
func WaitForPodReady(ctx context.Context, client *kubernetes.Clientset, namespace, podName string) error {
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

			if pod.Status.Phase == corev1.PodRunning {
				return nil
			}

			if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
				return fmt.Errorf("pod %s terminated early with phase %s", podName, pod.Status.Phase)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// AttachToPod attaches the current terminal to the Pod's container.
func AttachToPod(ctx context.Context, client *kubernetes.Clientset, config *rest.Config,
	namespace, podName, pgkContainerName string, tsq remotecommand.TerminalSizeQueue,
) error {
	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("attach")

	req.VersionedParams(&corev1.PodAttachOptions{
		Container: pgkContainerName,
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
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

// DeletePod deletes the specified Pod.
func DeletePod(ctx context.Context, client *kubernetes.Clientset, namespace, podName string) error {
	return client.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
}
