package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/xenos76/kubectl-netdrill/pkg/k8s"
	"github.com/xenos76/kubectl-netdrill/pkg/term"
)

var Command []string

var runCmd = &cobra.Command{
	Use:   "run [NAME]",
	Short: "Run an ephemeral pod for troubleshooting",
	Long:  `Create an ephemeral pod with the netdrill image and attach to it.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		podName := "netdrill"
		if len(args) > 0 {
			podName = args[0]
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle termination signals
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			cancel()
		}()

		client, config, err := k8s.GetClient(ConfigFlags)
		if err != nil {
			return fmt.Errorf("getting client: %w", err)
		}

		namespace, _, err := ConfigFlags.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			return fmt.Errorf("getting namespace: %w", err)
		}

		opts := k8s.PodOptions{
			Namespace:    namespace,
			PodName:      podName,
			Image:        Image,
			HostNetwork:  HostNetwork,
			NodeSelector: NodeSelector,
			Command:      Command,
		}

		fmt.Printf("Creating pod %s in namespace %s...\n", podName, namespace)
		_, err = k8s.CreatePod(ctx, client, opts)
		if err != nil {
			return fmt.Errorf("creating pod: %w", err)
		}

		// Ensure pod is deleted on exit
		defer func() {
			fmt.Printf("\nDeleting pod %s...\n", podName)
			if err := k8s.DeletePod(context.Background(), client, namespace, podName); err != nil {
				fmt.Printf("Error deleting pod: %v\n", err)
			}
		}()

		fmt.Printf("Waiting for pod %s to be ready...\n", podName)
		if err := k8s.WaitForPodReady(ctx, client, namespace, podName); err != nil {
			return fmt.Errorf("waiting for pod: %w", err)
		}

		fmt.Printf("Attaching to pod %s (container netdrill)...\n", podName)

		// Setup terminal for interactive session
		tsq := term.NewSizeQueue()
		go tsq.MonitorSize()
		defer tsq.Close()

		restore, err := term.SetRawMode()
		if err != nil {
			fmt.Printf("Error setting raw mode: %v\n", err)
		} else {
			defer restore()
		}

		if err := k8s.AttachToPod(ctx, client, config, namespace, podName, "netdrill", tsq); err != nil {
			return fmt.Errorf("attaching to pod: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringSliceVar(&Command, "command", nil, "Command to run in the container")
}
