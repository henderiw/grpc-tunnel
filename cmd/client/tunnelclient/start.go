package tunnelclient

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	tunnelAddress string
	certFile      string
	keyFile       string
	caFile        string
)

var startCmd = &cobra.Command{
	Use:          "start",
	Short:        "start the grpc tunnel client",
	Long:         "start the grpc tunnel client",
	Aliases:      []string{"start"},
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		//l := logr.NewStdoutLogger()
		zlog := zap.New(zap.UseDevMode(debug), zap.JSONEncoder())
		zlog.Info("grpc tunnels client...")

		c := client.New(
			client.WithLogger(logging.NewLogrLogger(zlog)),
			client.WithAddress(tunnelAddress),
			client.WithCertFile(certFile),
			client.WithKeyFile(keyFile),
			client.WithCaFile(caFile),
		)
		if err := c.Start(); err != nil {
			return errors.Wrap(err, "Cannot start tunnel client")
		}

		for {
		}

		//return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringVarP(&tunnelAddress, "tunnel-address", "a", ":33333", "The address the grpc tunnel server")
	startCmd.Flags().StringVarP(&certFile, "cert-file", "c", "cert/serverCert.pem", "The certificate file.")
	startCmd.Flags().StringVarP(&keyFile, "key-file", "k", "cert/serverKey.pem", "The key file.")
	startCmd.Flags().StringVarP(&caFile, "ca-file", "", "", "The ca file.")
}
