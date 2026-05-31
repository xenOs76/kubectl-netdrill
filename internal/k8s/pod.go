package k8s

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// AttachURLGetter is a function variable for creating a REST request URL for attach operations.
var AttachURLGetter = func(client kubernetes.Interface, namespace, podName, containerName string) (*url.URL, error) {
	req := client.CoreV1().RESTClient().Post().
		Namespace(namespace).
		Resource("pods").
		Name(podName).
		SubResource("attach")

	req.VersionedParams(&corev1.PodAttachOptions{
		Container: containerName,
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}, scheme.ParameterCodec)

	return req.URL(), nil
}

// SPDYExecutorCreator is a function variable for remotecommand.NewSPDYExecutor to allow mocking in tests.
var SPDYExecutorCreator = remotecommand.NewSPDYExecutor

// PodOptions defines options for creating a troubleshooting Pod.
type PodOptions struct {
	Namespace      string
	PodName        string
	Image          string
	HostNetwork    bool
	Command        []string
	Args           []string
	NodeSelector   map[string]string
	ServiceAccount string
	Ports          []corev1.ContainerPort
	EnvVars        []corev1.EnvVar
	Owner          string
	Ticket         string
}

// CreatePod creates a new Pod with the specified options.
func CreatePod(ctx context.Context, client kubernetes.Interface, opts PodOptions) (*corev1.Pod, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.PodName,
			Namespace: opts.Namespace,
			Labels:    netdrill.PodLabels(opts.Owner, opts.Ticket),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            netdrill.ContainerNetdrill,
					Image:           opts.Image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Stdin:           true,
					TTY:             true,
					Command:         opts.Command,
					Args:            opts.Args,
					Ports:           opts.Ports,
					Env:             opts.EnvVars,
				},
			},
			RestartPolicy:      corev1.RestartPolicyNever,
			HostNetwork:        opts.HostNetwork,
			NodeSelector:       opts.NodeSelector,
			ServiceAccountName: opts.ServiceAccount,
		},
	}

	ensureEKSToken(&pod.Spec)

	return client.CoreV1().Pods(opts.Namespace).Create(ctx, pod, metav1.CreateOptions{})
}

// ensureEKSToken adds a projected service account token volume if EKS IRSA env vars are present.
// This is a generic way to support IRSA even if the EKS webhook doesn't trigger.
func ensureEKSToken(spec *corev1.PodSpec) {
	if len(spec.Containers) == 0 {
		return
	}

	tokenPath, hasRole := getEKSTokenConfig(spec)

	if tokenPath == "" && !hasRole {
		return
	}

	if tokenPath == "" {
		tokenPath = "/var/run/secrets/eks.amazonaws.com/serviceaccount/token"
	}

	addEKSTokenVolume(spec, tokenPath)
}

func getEKSTokenConfig(spec *corev1.PodSpec) (string, bool) {
	var tokenPath string

	var hasRole bool

	for _, env := range spec.Containers[0].Env {
		if env.Name == "AWS_WEB_IDENTITY_TOKEN_FILE" {
			tokenPath = env.Value
		}

		if env.Name == "AWS_ROLE_ARN" {
			hasRole = true
		}
	}

	return tokenPath, hasRole
}

func addEKSTokenVolume(spec *corev1.PodSpec, tokenPath string) {
	volumeName := "aws-iam-token"
	volumeExists := false

	for _, v := range spec.Volumes {
		if v.Name == volumeName {
			volumeExists = true

			break
		}
	}

	if !volumeExists {
		expiration := int64(86400)
		tokenFile := path.Base(tokenPath)

		spec.Volumes = append(spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{
						{
							ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
								Audience:          "sts.amazonaws.com",
								ExpirationSeconds: &expiration,
								Path:              tokenFile,
							},
						},
					},
				},
			},
		})
	}

	for _, vm := range spec.Containers[0].VolumeMounts {
		if vm.Name == volumeName {
			return
		}
	}

	spec.Containers[0].VolumeMounts = append(spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      volumeName,
		ReadOnly:  true,
		MountPath: path.Dir(tokenPath),
	})
}

// WaitForPodReady waits until the Pod is in Running state.
var WaitForPodReady = func(ctx context.Context, client kubernetes.Interface, namespace, podName string) error {
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

			if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
				return fmt.Errorf("pod %s terminated early with phase %s", podName, pod.Status.Phase)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// AttachToPod attaches the current terminal to the Pod's container.
func AttachToPod(ctx context.Context, client kubernetes.Interface, config *rest.Config,
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

// MonitorPodStatus monitors the status of a pod until it is Running or Terminated.
var MonitorPodStatus = func(ctx context.Context, client kubernetes.Interface, namespace, podName string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return err
			}

			if pod.Status.Phase == corev1.PodRunning {
				return nil
			}

			if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
				return fmt.Errorf("pod %s terminated early with phase %s", podName, pod.Status.Phase)
			}
		}
	}
}

// DeletePod deletes the specified Pod.
func DeletePod(ctx context.Context, client kubernetes.Interface, namespace, podName string) error {
	err := client.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil && errors.IsNotFound(err) {
		return nil
	}

	return err
}
