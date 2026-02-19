package main

import (
	"github.com/spf13/cobra"
)

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion <bash|zsh|fish>",
		Short: "Generate shell completion script",
		Long: `Generate a shell completion script for forge.

To load completions:

  bash:
    source <(forge completion bash)

  zsh:
    echo 'source <(forge completion zsh)' >> ~/.zshrc

  fish:
    forge completion fish | source
    # To load on startup:
    forge completion fish > ~/.config/fish/completions/forge.fish
`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(cmd.OutOrStdout(), true)
			case "zsh":
				return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
			default:
				return cmd.Help()
			}
		},
	}
	return cmd
}
