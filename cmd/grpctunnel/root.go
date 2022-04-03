package grpctunnel

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	debug               bool
	certFile            string
	keyFile             string
	caFile              string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "grpctunnel",
	Short: "grpctunnel",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.SilenceUsage = true
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable debug mode")
	rootCmd.PersistentFlags().StringVarP(&certFile, "cert-file", "c", "cert/serverCert.pem", "The certificate file.")
	rootCmd.PersistentFlags().StringVarP(&keyFile, "key-file", "k", "cert/serverKey.pem", "The key file.")
	rootCmd.PersistentFlags().StringVarP(&caFile, "ca-file", "", "", "The ca file.")

}
