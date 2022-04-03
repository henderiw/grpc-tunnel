package grpctunnel

import (
	"github.com/henderiw/grpc-tunnel/internal/client"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	tunnelServerAddress string
	dialTarget          string
	dialTargetType      string
)

var clientStartCmd = &cobra.Command{
	Use:          "start",
	Short:        "start the grpc tunnel client",
	Long:         "start the grpc tunnel client",
	Aliases:      []string{"start"},
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		//l := logr.NewStdoutLogger()
		zlog := zap.New(zap.UseDevMode(debug), zap.JSONEncoder())
		zlog.Info("grpc tunnel client...")

		c := client.New(
			client.WithLogger(logging.NewLogrLogger(zlog)),
			client.WithTunnelServerAddress(tunnelServerAddress),
			client.WithDialTarget(dialTarget, dialTargetType),
			client.WithCertFile(certFile),
			client.WithKeyFile(keyFile),
			client.WithCaFile(caFile),
		)
		if err := c.Start(); err != nil {
			return errors.Wrap(err, "Cannot start tunnel client")
		}

		// block
		for {
		}
	},
}

func init() {
	clientCmd.AddCommand(clientStartCmd)
	clientStartCmd.Flags().StringVarP(&tunnelServerAddress, "tunnel-server-address", "s", "", "The address of the grpc tunnel server")
	clientStartCmd.Flags().StringVarP(&dialTarget, "dial-target", "t", "", "The remote target to dial")
	clientStartCmd.Flags().StringVarP(&dialTargetType, "dial-target-type", "", "SSH", "the type of protocol e.g. SSH or GNMI")

}
