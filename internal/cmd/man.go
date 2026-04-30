package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var manPagesDestDir string

var manCmd = &cobra.Command{
	Use:    "man [command]",
	Short:  "Generate man pages",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		header := &doc.GenManHeader{
			Title:   "KUBECTL-NETDRILL",
			Section: "1",
		}

		target := cmd.Root()

		if len(args) > 0 {
			var err error

			target, _, err = cmd.Root().Find(args)
			if err != nil {
				return err
			}
		}

		return doc.GenManTree(target, header, manPagesDestDir)
	},
}

func init() {
	rootCmd.AddCommand(manCmd)
	manCmd.Flags().StringVar(&manPagesDestDir, "dest-dir",
		".", "Destination directory for the man pages files")
}
