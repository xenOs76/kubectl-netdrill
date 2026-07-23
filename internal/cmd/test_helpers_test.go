package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// resetCmdState clears sticky cobra args/flags left by prior Execute calls
// (e.g. --help, --version) and restores Silence* defaults.
func resetCmdState() {
	rootCmd.SetArgs(nil)
	rootCmd.SilenceErrors = false
	rootCmd.SilenceUsage = false

	var resetFlags func(c *cobra.Command)

	resetFlags = func(c *cobra.Command) {
		resetFlagSet := func(fs *pflag.FlagSet) {
			if fs == nil {
				return
			}

			fs.VisitAll(func(f *pflag.Flag) {
				if !f.Changed && f.Value.String() == f.DefValue {
					return
				}

				_ = f.Value.Set(f.DefValue)
				f.Changed = false
			})
		}

		resetFlagSet(c.Flags())
		resetFlagSet(c.PersistentFlags())

		for _, sub := range c.Commands() {
			resetFlags(sub)
		}
	}
	resetFlags(rootCmd)
}
