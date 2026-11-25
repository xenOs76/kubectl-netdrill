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

// Command is the command to run in the container.
var Command []string

var runCmd = &cobra.Command{
	Use:   "run [NAME]",
	Short: "Run an ephemeral pod for troubleshooting",
	Long:  `Create an ephemeral pod with the netdrill image and attach to it.`,
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

		namespace, _, err := ConfigFlags.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			return fmt.Errorf("getting namespace: %w", err)
		}

		opts := k8s.PodOptions{
			Namespace:       namespace,
			PodName:         podName,
			Image:           Image,
			ImagePullPolicy: pullPolicy,
			HostNetwork:     HostNetwork,
			NodeSelector:    NodeSelector,
			Command:         Command,
		}

		verifiedImg, err := verifyAndPinImage(cmd.Context(), opts.Image, cmd.ErrOrStderr())
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

		_, _ = fmt.Printf("Creating pod %s in namespace %s...\n", podName, namespace)

		_, err = k8s.CreatePod(ctx, client, opts)
		if err != nil {
			return fmt.Errorf("creating pod: %w", err)
		}

		// Ensure pod is deleted on exit
		defer func() {
			_, _ = fmt.Printf("\nDeleting pod %s...\n", podName)

			if err := k8s.DeletePod(context.Background(), client, namespace, podName); err != nil {
				_, _ = fmt.Printf("Error deleting pod: %v\n", err)
			}
		}()

		_, _ = fmt.Printf("Waiting for pod %s to be ready...\n", podName)

		if err := k8s.WaitForPodReady(ctx, client, namespace, podName); err != nil {
			return fmt.Errorf("waiting for pod: %w", err)
		}

		_, _ = fmt.Printf("Attaching to pod %s (container netdrill)...\n", podName)

		// Setup terminal for interactive session
		tsq := term.NewSizeQueue()

		go tsq.MonitorSize(ctx)
		defer tsq.Close()

		restore, err := term.RawModeSetter()
		if err != nil {
			_, _ = fmt.Printf("Error setting raw mode: %v\n", err)
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
	runCmd.Flags().BoolVar(&HostNetwork, "host-network", false, "Use host networking")
	runCmd.Flags().StringToStringVar(&NodeSelector, "node-selector", map[string]string{},
		"node labels to use as a node selector for scheduling the netdrill pod "+
			"(e.g. kubernetes.io/os=linux; can be used multiple times)")

	_ = runCmd.RegisterFlagCompletionFunc("node-selector", completeNodeSelector)
}
