package tunnelserver

import (
	"github.com/henderiw/grpc-tunnel/internal/server"
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
	Short:        "start the grpc tunnel server",
	Long:         "start the grpc tunnel server",
	Aliases:      []string{"start"},
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		//l := logr.NewStdoutLogger()
		zlog := zap.New(zap.UseDevMode(debug), zap.JSONEncoder())
		zlog.Info("grpc tunnels server...")
		//zlogger, err := zap.NewProduction()
		//if err != nil {
		//	return err
		//}
		//defer zlogger.Sync()

		s := server.New(
			server.WithLogger(logging.NewLogrLogger(zlog)),
			server.WithAddress(tunnelAddress),
			server.WithCertFile(certFile),
			server.WithKeyFile(keyFile),
			server.WithCaFile(caFile),
		)
		if err := s.Start(); err != nil {
			return errors.Wrap(err, "Cannot start tunnel server")
		}

		for {}

		//return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringVarP(&tunnelAddress, "tunnel-address", "a", ":57401", "The address the grpc tunnel server")
	startCmd.Flags().StringVarP(&certFile, "cert-file", "c", "cert/serverCert.pem", "The certificate file.")
	startCmd.Flags().StringVarP(&keyFile, "key-file", "k", "cert/serverKey.pem", "The key file.")
	startCmd.Flags().StringVarP(&caFile, "ca-file", "", "", "The ca file.")
}
