package tunnelclient

import (
	"github.com/henderiw/grpc-tunnel/internal/client"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	tunnelAddress  string
	dialTarget     string
	dialTargetType string
	certFile       string
	keyFile        string
	caFile         string
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
			client.WithTunnelServerAddress(tunnelAddress),
			client.WithDialTarget(dialTarget, dialTargetType),
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
	startCmd.Flags().StringVarP(&tunnelAddress, "tunnel-server-address", "a", "34.79.210.188:57401", "The address of the grpc tunnel server")
	startCmd.Flags().StringVarP(&dialTarget, "dial-target", "t", "", "The remote target to dial")
	startCmd.Flags().StringVarP(&dialTargetType, "dial-target-type", "", "SSH", "the type of protocol e.g. SSH or GNMI")
	startCmd.Flags().StringVarP(&certFile, "cert-file", "c", "cert/serverCert.pem", "The certificate file.")
	startCmd.Flags().StringVarP(&keyFile, "key-file", "k", "cert/serverKey.pem", "The key file.")
	startCmd.Flags().StringVarP(&caFile, "ca-file", "", "", "The ca file.")
}
