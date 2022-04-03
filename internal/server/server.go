package server

import (
	"context"
	"net"
	"sync"
	"time"

	tpb "github.com/openconfig/grpctunnel/proto/tunnel"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/pkg/errors"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"google.golang.org/grpc"
)

const (
	errStartGRPCTunnelServer = "cannot start grpc tunnel server"
)

// Option can be used to manipulate Options.
type Option func(GrpcTunnelServer)

func WithLogger(l logging.Logger) Option {
	return func(s GrpcTunnelServer) {
		s.WithLogger(l)
	}
}

func WithTunnelServerAddress(f string) Option {
	return func(s GrpcTunnelServer) {
		s.WithTunnelServerAddress(f)
	}
}

func WithCertFile(f string) Option {
	return func(s GrpcTunnelServer) {
		s.WithCertFile(f)
	}
}

func WithKeyFile(f string) Option {
	return func(s GrpcTunnelServer) {
		s.WithKeyFile(f)
	}
}

func WithCaFile(f string) Option {
	return func(s GrpcTunnelServer) {
		s.WithCaFile(f)
	}
}

type GrpcTunnelServer interface {
	WithLogger(logging.Logger)
	WithTunnelServerAddress(string)
	WithCertFile(string)
	WithKeyFile(string)
	WithCaFile(string)
	Start() error
}

type config struct {
	address    string
	inSecure   bool
	skipVerify bool
	caFile     string
	certFile   string
	keyFile    string
}

type GrpcTunnelServerImpl struct {
	ctx context.Context
	cfg *config

	// tunnel server
	grpcTunnelServer *grpc.Server
	tunnelServer     *tunnel.Server
	// tunnel targets
	ttm          *sync.RWMutex
	tunTargets   map[string]tunnel.Target
	tunTargetCfn map[string]context.CancelFunc

	// logger
	log logging.Logger
}

func New(opts ...Option) GrpcTunnelServer {
	x := &GrpcTunnelServerImpl{
		cfg: &config{
			//skipVerify: true,
			//inSecure:   true,
		},
		ttm:          new(sync.RWMutex),
		tunTargets:   make(map[string]tunnel.Target),
		tunTargetCfn: make(map[string]context.CancelFunc),
	}

	for _, opt := range opts {
		opt(x)
	}

	return x
}

func (x *GrpcTunnelServerImpl) WithLogger(l logging.Logger) {
	x.log = l
}

func (x *GrpcTunnelServerImpl) WithTunnelServerAddress(a string) {
	x.cfg.address = a
}

func (x *GrpcTunnelServerImpl) WithCertFile(f string) {
	x.cfg.certFile = f
}

func (x *GrpcTunnelServerImpl) WithKeyFile(f string) {
	x.cfg.keyFile = f
}

func (x *GrpcTunnelServerImpl) WithCaFile(f string) {
	x.cfg.caFile = f
}

func (x *GrpcTunnelServerImpl) Start() error {
	errChannel := make(chan error)
	go func() {
		if err := x.run(); err != nil {
			errChannel <- errors.Wrap(err, errStartGRPCTunnelServer)
		}
		errChannel <- nil
	}()
	//fmt.Printf("Error start grpc tunnel server: %v\n", errChannel)
	return nil
}

func (x *GrpcTunnelServerImpl) run() error {
	var err error
	var opts []grpc.ServerOption
	if len(x.cfg.caFile) == 0 {
		opts, err = tunnel.ServerTLSCredsOpts(x.cfg.certFile, x.cfg.keyFile)
	} else {
		opts, err = tunnel.ServermTLSCredsOpts(x.cfg.certFile, x.cfg.keyFile, x.cfg.caFile)
	}
	if err != nil {
		x.log.Debug("failed creating grpc server option", "error", err)
		return err
	}
	// create a grpc server
	x.grpcTunnelServer = grpc.NewServer(opts...)

	// create a grpc tunnel server
	x.tunnelServer, err = tunnel.NewServer(tunnel.ServerConfig{
		AddTargetHandler:    x.tunServerAddTargetHandler,
		DeleteTargetHandler: x.tunServerDeleteTargetHandler,
		//RegisterHandler:     x.tunServerRegisterHandler,
		//Handler:             x.tunServerHandler,
	})
	if err != nil {
		x.log.Debug("failed creating tunnel server", "error", err)
		return err
	}

	// register the tunnel service with the grpc server
	tpb.RegisterTunnelServer(x.grpcTunnelServer, x.tunnelServer)

	var l net.Listener
	network := "tcp"
	addr := x.cfg.address

	// start the grpc tunnel server listener
	ctx, cancel := context.WithCancel(context.Background())
	for {
		l, err = net.Listen(network, addr)
		if err != nil {
			x.log.Debug("failed to start gRPC tunnel server listener", "error", err)
			time.Sleep(time.Second)
			continue
		}
		break
	}
	// serve the grpc tunnel server
	x.log.Debug("started grpc tunnel server", "address", addr)
	go func() {
		err = x.grpcTunnelServer.Serve(l)
		if err != nil {
			x.log.Debug("gRPC tunnel server shutdown", "error", err)
			//a.Logger.Printf("gRPC tunnel server shutdown: %v", err)
		}
		cancel()
	}()
	defer x.grpcTunnelServer.Stop()
	for range ctx.Done() {
	}
	return ctx.Err()
}

func (x *GrpcTunnelServerImpl) tunServerAddTargetHandler(tt tunnel.Target) error {
	x.log.Debug("tunServerAddTargetHandler", "target", tt)
	x.ttm.Lock()
	defer x.ttm.Unlock()
	x.tunTargets[tt.ID] = tt
	return nil
}

func (x *GrpcTunnelServerImpl) tunServerDeleteTargetHandler(tt tunnel.Target) error {
	x.log.Debug("tunServerDeleteTargetHandler", "target", tt)
	x.ttm.Lock()
	defer x.ttm.Unlock()
	if cfn, ok := x.tunTargetCfn[tt.ID]; ok {
		cfn()
		delete(x.tunTargetCfn, tt.ID)
		delete(x.tunTargets, tt.ID)
	}
	return nil
}

/*
func (x *GrpcTunnelServerImpl) tunServerRegisterHandler(ss tunnel.ServerSession) error {
	x.log.Debug("tunServerRegisterHandler", "target", ss)
	return nil
}

func (x *GrpcTunnelServerImpl) tunServerHandler(ss tunnel.ServerSession, rwc io.ReadWriteCloser) error {
	x.log.Debug("tunServerHandler", "target", ss)
	return nil
}
*/
