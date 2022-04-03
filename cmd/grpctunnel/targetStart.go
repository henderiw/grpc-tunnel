package grpctunnel

import (
	"github.com/henderiw/grpc-tunnel/internal/target"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	targetConfigFile string
)

var targetStartCmd = &cobra.Command{
	Use:          "start",
	Short:        "start the grpc tunnel target",
	Long:         "start the grpc tunnel target",
	Aliases:      []string{"start"},
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		zlog := zap.New(zap.UseDevMode(debug), zap.JSONEncoder())
		zlog.Info("grpc tunnel target...")

		s := target.New(
			target.WithLogger(logging.NewLogrLogger(zlog)),
			target.WithTunnelTargetConfigFile(targetConfigFile),
		)
		if err := s.Start(); err != nil {
			zlog.Error(err, "Cannot start tunnel target")
			return errors.Wrap(err, "Cannot start tunnel target")
		}
		// block
		for {
		}
	},
}

func init() {
	targetCmd.AddCommand(targetStartCmd)
	targetStartCmd.Flags().StringVarP(&targetConfigFile, "targetConfigFile", "t", "examples/target.cfg", "The configFile which specifies the tunnelServer and tunnelTargets")
}
