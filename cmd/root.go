package cmd

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var (
	ConfigFlags  *genericclioptions.ConfigFlags
	DefaultImage = "ghcr.io/xenos76/netdrill:latest"
	Image        string
	NodeSelector map[string]string
	Version      = "dev"
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

	rootCmd.PersistentFlags().StringVarP(&Image, "image", "i", DefaultImage, "The netdrill image to use")
	rootCmd.PersistentFlags().StringToStringVar(&NodeSelector,
		"node-selector", map[string]string{}, "node labels to use as a node selector for scheduling the netdrill pod (e.g. kubernetes.io/os=linux)")
}
