package grpctunnel

import (
	"github.com/henderiw/grpc-tunnel/internal/server"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var serverStartCmd = &cobra.Command{
	Use:          "start",
	Short:        "start the grpc tunnel server",
	Long:         "start the grpc tunnel server",
	Aliases:      []string{"start"},
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		zlog := zap.New(zap.UseDevMode(debug), zap.JSONEncoder())
		zlog.Info("grpc tunnel server...")

		s := server.New(
			server.WithLogger(logging.NewLogrLogger(zlog)),
			server.WithTunnelServerAddress(tunnelServerAddress),
			server.WithCertFile(certFile),
			server.WithKeyFile(keyFile),
			server.WithCaFile(caFile),
		)
		if err := s.Start(); err != nil {
			return errors.Wrap(err, "Cannot start tunnel server")
		}

		for {
		}
	},
}

func init() {
	serverCmd.AddCommand(serverStartCmd)
	serverStartCmd.Flags().StringVarP(&dialTarget, "dial-target", "", "target1", "The remote target to dial")
	serverStartCmd.Flags().StringVarP(&dialTargetType, "dial-target-type", "", "SSH", "the type of protocol e.g. SSH or GNMI")

}