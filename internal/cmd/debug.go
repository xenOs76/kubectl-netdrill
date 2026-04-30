package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	"github.com/xenos76/kubectl-netdrill/internal/term"
)

// TargetContainer is the name of the container into which to share the process namespace.
var TargetContainer string

var debugCmd = &cobra.Command{
	Use:   "debug [POD]",
	Short: "Debug an existing pod with an ephemeral container",
	Long:  `Add a temporary container with the netdrill image to an existing pod.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		podName := "netdrill"
		if len(args) > 0 {
			podName = args[0]
		}

		containerName := "netdrill-debug"

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle termination signals
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		go func() {
			<-sigChan
			cancel()
		}()

		client, config, err := k8s.ClientProvider(ConfigFlags)
		if err != nil {
			return fmt.Errorf("getting client: %w", err)
		}

		namespace, _, err := ConfigFlags.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			return fmt.Errorf("getting namespace: %w", err)
		}

		opts := k8s.EphemeralOptions{
			Namespace:     namespace,
			PodName:       podName,
			ContainerName: containerName,
			Image:         Image,
			TargetProcess: TargetContainer,
		}

		fmt.Printf("Adding ephemeral container %s to pod %s in namespace %s...\n", containerName, podName, namespace)

		err = k8s.AddEphemeralContainer(ctx, client, opts)
		if err != nil {
			return fmt.Errorf("adding ephemeral container: %w", err)
		}

		fmt.Printf("Waiting for ephemeral container %s to be ready...\n", containerName)

		if err := k8s.WaitForEphemeralContainerReady(ctx, client, namespace, podName, containerName); err != nil {
			return fmt.Errorf("waiting for ephemeral container: %w", err)
		}

		fmt.Printf("Attaching to ephemeral container %s...\n", containerName)

		// Setup terminal for interactive session
		tsq := term.NewSizeQueue()

		go tsq.MonitorSize(ctx)
		defer tsq.Close()

		restore, err := term.RawModeSetter()
		if err != nil {
			fmt.Printf("Error setting raw mode: %v\n", err)
		} else {
			defer restore()
		}

		if err := k8s.AttachToEphemeralContainer(ctx, client, config,
			namespace, podName, containerName, tsq); err != nil {
			return fmt.Errorf("attaching to ephemeral container: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(debugCmd)

	debugCmd.Flags().StringVar(&TargetContainer, "target", "",
		"The name of the container into which to share the process namespace")
}
