package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
)

var (
	// Replicas is the number of pod replicas for the deployment.
	Replicas int32
	// CPURequest is the CPU resource request.
	CPURequest string
	// MemoryRequest is the memory resource request.
	MemoryRequest string
	// CPULimit is the CPU resource limit.
	CPULimit string
	// MemoryLimit is the memory resource limit.
	MemoryLimit string
	// Labels are additional labels for the deployment.
	Labels map[string]string
)

var deploymentCmd = &cobra.Command{
	Use:   "deployment [NAME]",
	Short: "Create a persistent deployment for troubleshooting",
	Long: `Create a persistent deployment with the netdrill image.
Unlike 'run', this deployment is not automatically deleted.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		deployName := "netdrill"
		if len(args) > 0 {
			deployName = args[0]
		}

		appLabel := deployName

		ctx := context.Background()

		client, _, err := k8s.ClientProvider(ConfigFlags)
		if err != nil {
			return fmt.Errorf("getting client: %w", err)
		}

		namespace, _, err := ConfigFlags.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			return fmt.Errorf("getting namespace: %w", err)
		}

		opts := k8s.DeploymentOptionsFromConfig(netdrill.DeploymentConfig{
			PodConfig: netdrill.PodConfig{
				Namespace:      namespace,
				PodName:        deployName,
				Image:          Image,
				HostNetwork:    HostNetwork,
				NodeSelector:   NodeSelector,
				ServiceAccount: ServiceAccount,
				Ports:          Ports,
				EnvVars:        EnvVars,
			},
			Replicas:      &Replicas,
			Labels:        Labels,
			AppLabel:      appLabel,
			CPURequest:    CPURequest,
			MemoryRequest: MemoryRequest,
			CPULimit:      CPULimit,
			MemoryLimit:   MemoryLimit,
		})

		fmt.Printf("Creating deployment %s in namespace %s...\n", deployName, namespace)

		_, err = k8s.CreateDeployment(ctx, client, opts)
		if err != nil {
			return fmt.Errorf("creating deployment: %w", err)
		}

		fmt.Printf("Deployment %s created successfully.\n", deployName)
		fmt.Printf("Use 'kubectl get pods -l app=%s' to find the pods.\n", appLabel)
		fmt.Printf("Use 'kubectl delete deployment %s -n %s' to delete it.\n", deployName, namespace)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(deploymentCmd)

	deploymentCmd.Flags().Int32Var(&Replicas, "replicas", 1, "Number of replicas")
	deploymentCmd.Flags().StringVar(&CPURequest, "cpu-request", "", "CPU request (e.g. 100m)")
	deploymentCmd.Flags().StringVar(&MemoryRequest, "memory-request", "", "Memory request (e.g. 128Mi)")
	deploymentCmd.Flags().StringVar(&CPULimit, "cpu-limit", "", "CPU limit (e.g. 200m)")
	deploymentCmd.Flags().StringVar(&MemoryLimit, "memory-limit", "", "Memory limit (e.g. 256Mi)")
	deploymentCmd.Flags().StringToStringVar(&Labels, "labels", map[string]string{},
		"Additional labels (e.g. key1=val1,key2=val2)")

	deploymentCmd.Flags().StringVar(&ServiceAccount, "service-account", "",
		"ServiceAccount to use for the deployment")
	deploymentCmd.Flags().Int32SliceVar(&Ports, "port", []int32{},
		"Ports to expose on the container (e.g., --port 80)")
	deploymentCmd.Flags().StringToStringVar(&EnvVars, "env", map[string]string{},
		"Environment variables (e.g., --env KEY=VALUE)")
	deploymentCmd.Flags().BoolVar(&HostNetwork, "host-network", false, "Use host networking")
	deploymentCmd.Flags().StringToStringVar(&NodeSelector, "node-selector", map[string]string{},
		"node labels to use as a node selector for scheduling the netdrill pod (e.g. kubernetes.io/os=linux)")
}
