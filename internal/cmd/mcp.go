package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/xenos76/kubectl-netdrill/internal/image"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	mcpsrv "github.com/xenos76/kubectl-netdrill/internal/mcp"
)

var (
	mcpOwner               string
	mcpExecTimeout         time.Duration
	mcpMaxOutputBytes      int64
	mcpInsecureAllowAnyPod bool
	mcpResolveImage        bool
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run Model Context Protocol server for AI agents",
	Long: `Start an MCP server over stdin/stdout that exposes kubectl-netdrill operations
to AI clients. Use --owner to bind create/delete/exec to pods you own.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()

		client, restConfig, err := k8s.ClientProvider(ConfigFlags)
		if err != nil {
			return fmt.Errorf("getting client: %w", err)
		}

		namespace, _, err := ConfigFlags.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			return fmt.Errorf("getting namespace: %w", err)
		}

		owner := mcpOwner
		if owner == "" {
			owner = os.Getenv("USER")
		}

		if owner == "" {
			return errors.New("--owner is required when USER is unset")
		}

		podImage := Image
		if mcpResolveImage {
			resolved, err := image.ResolveIfLatest(ctx, Image, nil)
			if err != nil {
				slog.Warn("netdrill image resolution failed; using configured image",
					"image", Image, "err", err)
			} else if resolved != Image {
				slog.Info("resolved netdrill image", "from", Image, "to", resolved)
				podImage = resolved
			}
		}

		deps := &mcpsrv.Deps{
			Client: client,
			Config: restConfig,
			Cfg: mcpsrv.Config{
				Image:               podImage,
				DefaultNamespace:    namespace,
				Owner:               owner,
				ExecTimeout:         mcpExecTimeout,
				MaxOutputBytes:      mcpMaxOutputBytes,
				InsecureAllowAnyPod: mcpInsecureAllowAnyPod,
			},
			Guard: mcpsrv.Guard{
				Owner:       owner,
				AllowAnyPod: mcpInsecureAllowAnyPod,
				MaxOutput:   mcpMaxOutputBytes,
			},
		}

		return mcpsrv.Run(ctx, Version, deps)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)

	mcpCmd.Flags().StringVar(&mcpOwner, "owner", "",
		"Owner identity stamped on created pods (default: $USER)")
	mcpCmd.Flags().DurationVar(&mcpExecTimeout, "exec-timeout", 120*time.Second,
		"Timeout for netdrill_pod_exec and netdrill_debug_exec")
	mcpCmd.Flags().Int64Var(&mcpMaxOutputBytes, "max-output-bytes", 1<<20,
		"Maximum combined stdout+stderr bytes per exec")
	mcpCmd.Flags().BoolVar(&mcpInsecureAllowAnyPod, "insecure-allow-any-pod", false,
		"Skip managed/owner label checks (dangerous)")
	mcpCmd.Flags().BoolVar(&mcpResolveImage, "resolve-image", true,
		"Resolve :latest to the highest semver tag on GHCR (falls back on error)")
}
