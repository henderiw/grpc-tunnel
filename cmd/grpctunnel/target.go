package grpctunnel

import "github.com/spf13/cobra"

// rootCmd represents the base command when called without any subcommands
var targetCmd = &cobra.Command{
	Use:   "target",
	Short: "target",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
	},
}

func init() {
	rootCmd.AddCommand(targetCmd)
}
