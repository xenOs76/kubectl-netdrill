package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: `To load completions:

Bash:

  $ source <(kubectl-netdrill completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ kubectl-netdrill completion bash > /etc/bash_completion.d/kubectl-netdrill
  # macOS:
  $ kubectl-netdrill completion bash > /usr/local/etc/bash_completion.d/kubectl-netdrill

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ kubectl-netdrill completion zsh > "${fpath[1]}/_kubectl-netdrill"

  # You will need to start a new shell for this setup to take effect.

Fish:

  $ kubectl-netdrill completion fish | source

  # To load completions for each session, execute once:
  $ kubectl-netdrill completion fish > ~/.config/fish/completions/kubectl-netdrill.fish

PowerShell:

  PS> kubectl-netdrill completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> kubectl-netdrill completion powershell > kubectl-netdrill.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			if err := cmd.Root().GenBashCompletionV2(os.Stdout, false); err != nil {
				return err
			}

			fmt.Println(`
# Additional completion bindings for direct binary invocations and aliases
if [[ $(type -t compopt) = "builtin" ]]; then
    complete -o default -F __start_kubectl-netdrill ./dist/kubectl-netdrill dist/kubectl-netdrill knetdrill
else
    complete -o default -o nospace -F __start_kubectl-netdrill ./dist/kubectl-netdrill dist/kubectl-netdrill knetdrill
fi`)
		case "zsh":
			if err := cmd.Root().GenZshCompletion(os.Stdout); err != nil {
				return err
			}

			fmt.Println("\ncompdef _kubectl-netdrill ./dist/kubectl-netdrill dist/kubectl-netdrill knetdrill")
		case "fish":
			if err := cmd.Root().GenFishCompletion(os.Stdout, true); err != nil {
				return err
			}

			fmt.Println("\ncomplete -c ./dist/kubectl-netdrill -w kubectl-netdrill\n" +
				"complete -c dist/kubectl-netdrill -w kubectl-netdrill\n" +
				"complete -c knetdrill -w kubectl-netdrill")
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		default:
			return fmt.Errorf("unsupported shell %s", args[0])
		}

		return nil
	},
}
