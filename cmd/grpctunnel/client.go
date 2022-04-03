package grpctunnel

import "github.com/spf13/cobra"

// rootCmd represents the base command when called without any subcommands
var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "client",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
	},
}

func init() {
	rootCmd.AddCommand(clientCmd)
}
