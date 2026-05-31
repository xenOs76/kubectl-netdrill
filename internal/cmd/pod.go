package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
)

var podCmd = &cobra.Command{
	Use:   "pod [NAME]",
	Short: "Create a persistent pod for troubleshooting",
	Long:  `Create a persistent pod with the netdrill image. Unlike 'run', this pod is not automatically deleted.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		podName := "netdrill"
		if len(args) > 0 {
			podName = args[0]
		}

		ctx := context.Background()

		client, _, err := k8s.ClientProvider(ConfigFlags)
		if err != nil {
			return fmt.Errorf("getting client: %w", err)
		}

		namespace, _, err := ConfigFlags.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			return fmt.Errorf("getting namespace: %w", err)
		}

		opts := k8s.PodOptionsFromConfig(netdrill.PodConfig{
			Namespace:      namespace,
			PodName:        podName,
			Image:          Image,
			HostNetwork:    HostNetwork,
			NodeSelector:   NodeSelector,
			ServiceAccount: ServiceAccount,
			Ports:          Ports,
			EnvVars:        EnvVars,
		})

		fmt.Printf("Creating pod %s in namespace %s...\n", podName, namespace)

		_, err = k8s.CreatePod(ctx, client, opts)
		if err != nil {
			return fmt.Errorf("creating pod: %w", err)
		}

		fmt.Printf("Pod %s created successfully.\n", podName)
		fmt.Printf("Use 'kubectl exec -it %s -n %s -- /bin/bash' to attach to it.\n", podName, namespace)
		fmt.Printf("Use 'kubectl delete pod %s -n %s' to delete it.\n", podName, namespace)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(podCmd)

	podCmd.Flags().StringVar(&ServiceAccount, "service-account", "",
		"ServiceAccount to use for the pod")
	podCmd.Flags().Int32SliceVar(&Ports, "port", []int32{},
		"Ports to expose on the container (e.g., --port 80)")
	podCmd.Flags().StringToStringVar(&EnvVars, "env", map[string]string{},
		"Environment variables (e.g., --env KEY=VALUE)")
	podCmd.Flags().BoolVar(&HostNetwork, "host-network", false, "Use host networking")
	podCmd.Flags().StringToStringVar(&NodeSelector, "node-selector", map[string]string{},
		"node labels to use as a node selector for scheduling the netdrill pod (e.g. kubernetes.io/os=linux)")
}
