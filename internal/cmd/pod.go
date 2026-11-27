package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
	"k8s.io/client-go/kubernetes"
)

var podCmd = &cobra.Command{
	Use:   "pod [NAME]",
	Short: "Create a persistent pod for troubleshooting",
	Long:  `Create a persistent pod with the netdrill image. Unlike 'run', this pod is not automatically deleted.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		isDryRun, format, err := validateDryRunOutput(DryRun, Output)
		if err != nil {
			return err
		}

		pullPolicy, err := validateImagePullPolicy(ImagePullPolicy)
		if err != nil {
			return err
		}

		podName := "netdrill"
		if len(args) > 0 {
			podName = args[0]
		}

		ctx := context.Background()

		var client kubernetes.Interface
		if !isDryRun || SameAs != "" {
			client, _, err = k8s.ClientProvider(ConfigFlags)
			if err != nil {
				return fmt.Errorf("getting client: %w", err)
			}
		}

		namespace, _, err := ConfigFlags.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			return fmt.Errorf("getting namespace: %w", err)
		}

		cfg := netdrill.PodConfig{
			Namespace:       namespace,
			PodName:         podName,
			Image:           Image,
			ImagePullPolicy: pullPolicy,
			HostNetwork:     HostNetwork,
			NodeSelector:    NodeSelector,
			ServiceAccount:  ServiceAccount,
			Ports:           Ports,
			EnvVars:         EnvVars,
			Labels:          Labels,
			CPURequest:      CPURequest,
			MemoryRequest:   MemoryRequest,
			CPULimit:        CPULimit,
			MemoryLimit:     MemoryLimit,
		}

		if SameAs != "" {
			target, err := k8s.ApplySameAsPodConfig(ctx, client, cmd, SameAs, SameAsContainer, namespace, &cfg)
			if err != nil {
				return fmt.Errorf("applying --same-as: %w", err)
			}

			if isDryRun {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Cloning operational settings from %s %s/%s...\n",
					target.Kind, target.Namespace, target.Name)
			} else {
				_, _ = fmt.Printf("Cloning operational settings from %s %s/%s...\n",
					target.Kind, target.Namespace, target.Name)
			}
		}

		if Secure {
			if err := k8s.ApplySecurePodConfig(&cfg); err != nil {
				return err
			}
		}

		opts := k8s.PodOptionsFromConfig(cfg)

		verifiedImg, err := verifyAndPinImage(ctx, opts.Image, cmd.ErrOrStderr())
		if err != nil {
			return err
		}

		opts.Image = verifiedImg

		if isDryRun {
			pod, err := k8s.BuildPod(opts)
			if err != nil {
				return fmt.Errorf("building pod: %w", err)
			}

			return printDryRunObject(cmd.OutOrStdout(), pod, format)
		}

		_, _ = fmt.Printf("Creating pod %s in namespace %s...\n", cfg.PodName, cfg.Namespace)

		_, err = k8s.CreatePod(ctx, client, opts)
		if err != nil {
			return fmt.Errorf("creating pod: %w", err)
		}

		_, _ = fmt.Printf("Pod %s created successfully.\n", cfg.PodName)
		_, _ = fmt.Printf("Use 'kubectl exec -it %s -n %s -- /bin/bash' to attach to it.\n",
			cfg.PodName, cfg.Namespace)
		_, _ = fmt.Printf("Use 'kubectl delete pod %s -n %s' to delete it.\n", cfg.PodName, cfg.Namespace)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(podCmd)

	podCmd.Flags().StringVar(&SameAs, "same-as", "",
		"Existing pod or deployment to clone operational settings from "+
			"(e.g. 'my-pod', 'pod/my-pod', 'deployment/my-deploy', 'ns/kind/name')")
	podCmd.Flags().StringVar(&SameAsContainer, "same-as-container", "",
		"Container name within the target workload to clone resources/env/ports from (defaults to first container)")
	podCmd.Flags().StringVar(&CPURequest, "cpu-request", "", "CPU request (e.g. 100m)")
	podCmd.Flags().StringVar(&MemoryRequest, "memory-request", "", "Memory request (e.g. 128Mi)")
	podCmd.Flags().StringVar(&CPULimit, "cpu-limit", "", "CPU limit (e.g. 200m)")
	podCmd.Flags().StringVar(&MemoryLimit, "memory-limit", "", "Memory limit (e.g. 256Mi)")

	podCmd.Flags().StringVar(&ServiceAccount, "service-account", "",
		"ServiceAccount to use for the pod")
	podCmd.Flags().Int32SliceVar(&Ports, "port", []int32{},
		"Ports to expose on the container (e.g., --port 80; can be used multiple times)")
	podCmd.Flags().StringToStringVar(&EnvVars, "env", map[string]string{},
		"Environment variables (e.g., --env KEY=VALUE; can be used multiple times)")
	podCmd.Flags().BoolVar(&HostNetwork, "host-network", false, "Use host networking")
	podCmd.Flags().BoolVar(&Secure, "secure", false,
		"Apply Kubernetes Restricted Pod Security Standards and hardening defaults")
	podCmd.Flags().StringToStringVar(&NodeSelector, "node-selector", map[string]string{},
		"node labels to use as a node selector for scheduling the netdrill pod "+
			"(e.g. kubernetes.io/os=linux; can be used multiple times)")
	podCmd.Flags().StringToStringVar(&Labels, "labels", map[string]string{},
		"Additional labels (e.g. key1=val1,key2=val2; can be used multiple times)")

	_ = podCmd.RegisterFlagCompletionFunc("same-as", completeSameAs)
	_ = podCmd.RegisterFlagCompletionFunc("same-as-container", completeSameAsContainer)
	_ = podCmd.RegisterFlagCompletionFunc("cpu-request", completeCPU)
	_ = podCmd.RegisterFlagCompletionFunc("cpu-limit", completeCPU)
	_ = podCmd.RegisterFlagCompletionFunc("memory-request", completeMemory)
	_ = podCmd.RegisterFlagCompletionFunc("memory-limit", completeMemory)
	_ = podCmd.RegisterFlagCompletionFunc("node-selector", completeNodeSelector)
	_ = podCmd.RegisterFlagCompletionFunc("service-account", completeServiceAccount)
	_ = podCmd.RegisterFlagCompletionFunc("labels", completeLabels)
	_ = podCmd.RegisterFlagCompletionFunc("port", completePort)
	_ = podCmd.RegisterFlagCompletionFunc("env", completeEnv)
}
