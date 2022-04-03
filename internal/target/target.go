package target

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/openconfig/grpctunnel/bidi"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/pkg/errors"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/encoding/prototext"

	tcpb "github.com/openconfig/grpctunnel/cmd/target/proto/config"
	tpb "github.com/openconfig/grpctunnel/proto/tunnel"
)

const (
	errStartGRPCTunnelTarget = "cannot start grpc tunnel target"

	// for setting retry backoff when waiting for target.
	retryBaseDelay     = time.Second
	retryMaxDelay      = time.Minute
	retryRandomization = 0.5
)

// Option can be used to manipulate Options.
type Option func(GrpcTunnelTarget)

func WithLogger(l logging.Logger) Option {
	return func(s GrpcTunnelTarget) {
		s.WithLogger(l)
	}
}

func WithTunnelTargetConfigFile(f string) Option {
	return func(s GrpcTunnelTarget) {
		s.WithTunnelTargetConfigFile(f)
	}
}

type GrpcTunnelTarget interface {
	WithLogger(logging.Logger)
	WithTunnelTargetConfigFile(string)
	Start() error
}

type GrpcTunnelTargetImpl struct {
	ctx     context.Context
	cfgFile string
	config  *tcpb.TunnelTargetConfig

	// targets
	targetMux sync.Mutex
	targets   map[tunnel.Target]struct{}

	// logger
	log logging.Logger
}

func New(opts ...Option) GrpcTunnelTarget {
	x := &GrpcTunnelTargetImpl{
		ctx: context.Background(),

		targets: make(map[tunnel.Target]struct{}),
	}

	for _, opt := range opts {
		opt(x)
	}

	return x
}

func (x *GrpcTunnelTargetImpl) WithLogger(l logging.Logger) {
	x.log = l
}

func (x *GrpcTunnelTargetImpl) WithTunnelTargetConfigFile(c string) {
	x.cfgFile = c
}

func (x *GrpcTunnelTargetImpl) Start() error {
	// validate and parse the config file
	if x.cfgFile == "" {
		return errors.New("config file missing, is required to start the grpctunnel target")
	}
	// Initialize configuration.
	buf, err := ioutil.ReadFile(x.cfgFile)
	if err != nil {
		return fmt.Errorf("could not read config file: %s, err: %s", x.cfgFile, err.Error())
	}
	x.config = &tcpb.TunnelTargetConfig{}
	if err := prototext.Unmarshal(buf, x.config); err != nil {
		return fmt.Errorf("could not parse configuration from: %s, err: %s", x.cfgFile, err.Error())
	}

	x.log.Debug("grpctunnel start", "config", x.config)

	// initialize the targets
	for _, target := range x.config.GetTunnelTarget() {
		t := tunnel.Target{ID: target.GetTarget(), Type: target.GetType()}
		x.targets[t] = struct{}{}
	}

	// start the grpctunnel target client
	errChannel := make(chan error)
	go func() {
		fmt.Println("test")
		if err := x.run(x.ctx); err != nil {
			errChannel <- errors.Wrap(err, errStartGRPCTunnelTarget)
		}
		errChannel <- nil
	}()
	return nil
}

func (x *GrpcTunnelTargetImpl) run(ctx context.Context) error {
	log := x.log.WithValues("tunnel server address", x.config.GetTunnelServerDefault()[0].GetTunnelServerAddress())
	log.Debug("grpctunnel target run...")

	// TODO add better certificate handling, right now self signed certificate
	opts, err := x.getGrpcOptions()
	if err != nil {
		return err
	}

	// dial to tunnel server with retry
	defer runtime.UnlockOSThread()
	var conn *grpc.ClientConn
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			var err error
			gnmiCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			conn, err = grpc.DialContext(gnmiCtx, x.config.GetTunnelServerDefault()[0].GetTunnelServerAddress(), opts...)
			if err != nil {
				log.Debug("failed to connect to grpc tunnel server", "error", err)
				time.Sleep(5 * time.Second)
				continue
			}
		}
		break
	}
	defer conn.Close()

	client, err := tunnel.NewClient(tpb.NewTunnelClient(conn), tunnel.ClientConfig{
		RegisterHandler: x.registerHandler(),
		Handler:         x.handler(),
	}, x.targets)
	if err != nil {
		log.Debug("failed to create tunnel target client", "error", err)
		return err
	}
	log.Debug("tunnel target client create...", "targets", x.targets)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Register and start listening.
	if err := client.Register(ctx); err != nil {
		return err
	}

	/*
		for t := range x.targets {
			if err := client.NewTarget(t); err != nil {
				log.Debug("register target failed", "target.ID", t.ID, "target.Type", t.Type, "error", err)
				return err
			}
			log.Debug("register target succeeded", "target.ID", t.ID, "target.Type", t.Type)
		}
	*/

	log.Debug("tunnel target client registered...")
	client.Start(ctx)
	return client.Error()
}

func (x *GrpcTunnelTargetImpl) getGrpcOptions() ([]grpc.DialOption, error) {
	opts := []grpc.DialOption{
		grpc.WithBlock(),
	}

	cred := x.config.GetTunnelServerDefault()[0].GetCredentials().GetTls()
	switch {
	case len(cred.GetCertFile()) == 0 && len(cred.GetKeyFile()) == 0 && len(cred.GetCaFile()) == 0:
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	case len(cred.GetCertFile()) == 0 || len(cred.GetKeyFile()) == 0:
		o, err := tunnel.DialTLSCredsOpts(cred.GetCaFile())
		if err != nil {
			return nil, err
		}
		opts = append(opts, o...)
	default:
		o, err := tunnel.DialmTLSCredsOpts(cred.GetCertFile(), cred.GetKeyFile(), cred.GetCaFile())
		if err != nil {
			return nil, err
		}
		opts = append(opts, o...)
	}

	return opts, nil
}

func (x *GrpcTunnelTargetImpl) registerHandler() func(t tunnel.Target) error {
	return func(t tunnel.Target) error {
		x.log.Debug("grpctunnel target registerHandler", "tunnel.Target", t)
		for _, target := range x.config.GetTunnelTarget() {
			if t.ID == target.GetTarget() {
				return nil
			}
		}
		return fmt.Errorf("client cannot handle: %s", t.ID)
	}
}

func (x *GrpcTunnelTargetImpl) handler() func(t tunnel.Target, i io.ReadWriteCloser) error {
	return func(t tunnel.Target, i io.ReadWriteCloser) error {
		x.log.Debug("grpctunnel target handler", "tunnel.Target", t)
		var dialAddr string
		for _, target := range x.config.GetTunnelTarget() {
			if t.ID == target.GetTarget() && t.Type == target.GetType() {
				dialAddr = target.GetDialAddress()
				break
			}
		}
		if len(dialAddr) == 0 {
			return fmt.Errorf("not maching dial port found for target: %s|%s", t.ID, t.Type)
		}

		conn, err := net.Dial("tcp", dialAddr)
		if err != nil {
			return fmt.Errorf("failed to dial %s: %v", dialAddr, err)
		}

		if err = bidi.Copy(i, conn); err != nil {
			log.Printf("error from bidi copy: %v", err)
		}
		return nil
	}
}
