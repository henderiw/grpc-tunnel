package server

import (
	"context"
	"io"
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

func WithAddress(f string) Option {
	return func(s GrpcTunnelServer) {
		s.WithAddress(f)
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
	WithAddress(string)
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
	s := &GrpcTunnelServerImpl{
		cfg: &config{
			//skipVerify: true,
			//inSecure:   true,
		},
		ttm:          new(sync.RWMutex),
		tunTargets:   make(map[string]tunnel.Target),
		tunTargetCfn: make(map[string]context.CancelFunc),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *GrpcTunnelServerImpl) WithLogger(l logging.Logger) {
	s.log = l
}

func (s *GrpcTunnelServerImpl) WithAddress(a string) {
	s.cfg.address = a
}

func (s *GrpcTunnelServerImpl) WithCertFile(f string) {
	s.cfg.certFile = f
}

func (s *GrpcTunnelServerImpl) WithKeyFile(f string) {
	s.cfg.keyFile = f
}

func (s *GrpcTunnelServerImpl) WithCaFile(f string) {
	s.cfg.caFile = f
}

func (s *GrpcTunnelServerImpl) Start() error {
	errChannel := make(chan error)
	go func() {
		if err := s.run(); err != nil {
			errChannel <- errors.Wrap(err, errStartGRPCTunnelServer)
		}
		errChannel <- nil
	}()
	//fmt.Printf("Error start grpc tunnel server: %v\n", errChannel)
	return nil
}

func (s *GrpcTunnelServerImpl) run() error {
	var err error
	s.tunnelServer, err = tunnel.NewServer(tunnel.ServerConfig{
		AddTargetHandler:    s.tunServerAddTargetHandler,
		DeleteTargetHandler: s.tunServerDeleteTargetHandler,
	})
	if err != nil {
		s.log.Debug("failed creating tunnel server", "error", err)
		return err
	}

	var opts []grpc.ServerOption
	if len(s.cfg.caFile) == 0 {
		opts, err = tunnel.ServerTLSCredsOpts(s.cfg.certFile, s.cfg.keyFile)
	} else {
		opts, err = tunnel.ServermTLSCredsOpts(s.cfg.certFile, s.cfg.keyFile, s.cfg.caFile)
	}
	if err != nil {
		s.log.Debug("failed creating grpc server option", "error", err)
		return err
	}
	s.grpcTunnelServer = grpc.NewServer(opts...)

	// register the tunnel service with the grpc server
	tpb.RegisterTunnelServer(s.grpcTunnelServer, s.tunnelServer)

	var l net.Listener
	network := "tcp"
	addr := s.cfg.address

	ctx, cancel := context.WithCancel(context.Background())
	for {
		l, err = net.Listen(network, addr)
		if err != nil {
			s.log.Debug("failed to start gRPC tunnel server listener", "error", err)
			time.Sleep(time.Second)
			continue
		}
		break
	}
	s.log.Debug("started grpc tunnel server", "address", addr)
	go func() {
		err = s.grpcTunnelServer.Serve(l)
		if err != nil {
			s.log.Debug("gRPC tunnel server shutdown", "error", err)
			//a.Logger.Printf("gRPC tunnel server shutdown: %v", err)
		}
		cancel()
	}()
	defer s.grpcTunnelServer.Stop()
	for range ctx.Done() {
	}
	return ctx.Err()
}

func (s *GrpcTunnelServerImpl) tunServerAddTargetHandler(tt tunnel.Target) error {
	s.log.Debug("tunServerAddTargetHandler", "target", tt)
	s.ttm.Lock()
	defer s.ttm.Unlock()
	s.tunTargets[tt.ID] = tt
	return nil
}

func (s *GrpcTunnelServerImpl) tunServerDeleteTargetHandler(tt tunnel.Target) error {
	s.log.Debug("tunServerDeleteTargetHandler", "target", tt)
	s.ttm.Lock()
	defer s.ttm.Unlock()
	if cfn, ok := s.tunTargetCfn[tt.ID]; ok {
		cfn()
		delete(s.tunTargetCfn, tt.ID)
		delete(s.tunTargets, tt.ID)
	}
	return nil
}

func (s *GrpcTunnelServerImpl) tunServerRegisterHandler(ss tunnel.ServerSession) error {
	return nil
}

func (s *GrpcTunnelServerImpl) tunServerHandler(ss tunnel.ServerSession, rwc io.ReadWriteCloser) error {
	return nil
}
