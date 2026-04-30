package cmd

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var (
	// ConfigFlags holds the configuration flags for the Kubernetes client.
	ConfigFlags *genericclioptions.ConfigFlags
	// DefaultImage is the default image used for troubleshooting containers.
	DefaultImage = "ghcr.io/xenos76/netdrill:latest"
	// Image is the container image to use.
	Image string
	// NodeSelector defines labels for scheduling pods.
	NodeSelector map[string]string
	// Version is the application version.
	Version = "dev"
	// ServiceAccount is the name of the service account to use for pods.
	ServiceAccount string
	// Ports are the container ports to expose.
	Ports []int32
	// EnvVars are the environment variables for the container.
	EnvVars map[string]string
	// HostNetwork specifies if host networking should be used.
	HostNetwork bool
)

var rootCmd = &cobra.Command{
	Use:   "kubectl-netdrill",
	Short: "kubectl-netdrill is a network troubleshooting plugin",
	Long: `A kubectl plugin for running network troubleshooting tools using the netdrill image.
Inspired by kubectl-netshoot.`,
	Version: Version,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

// Execute defines the entry point for the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	ConfigFlags = genericclioptions.NewConfigFlags(true)
	ConfigFlags.AddFlags(rootCmd.PersistentFlags())

	rootCmd.PersistentFlags().StringVarP(&Image,
		"image",
		"i",
		DefaultImage,
		"The netdrill image to use",
	)

	rootCmd.AddCommand(completionCmd)
}
