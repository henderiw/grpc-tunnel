package client

import (
	"context"
	"sync"

	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/pkg/errors"
	"github.com/yndd/ndd-runtime/pkg/logging"
)

const (
	errStartGRPCTunnelClient = "cannot start grpc tunnel client"
)

// Option can be used to manipulate Options.
type Option func(GrpcTunnelClient)

func WithLogger(l logging.Logger) Option {
	return func(s GrpcTunnelClient) {
		s.WithLogger(l)
	}
}

func WithAddress(f string) Option {
	return func(s GrpcTunnelClient) {
		s.WithAddress(f)
	}
}

func WithCertFile(f string) Option {
	return func(s GrpcTunnelClient) {
		s.WithCertFile(f)
	}
}

func WithKeyFile(f string) Option {
	return func(s GrpcTunnelClient) {
		s.WithKeyFile(f)
	}
}

func WithCaFile(f string) Option {
	return func(s GrpcTunnelClient) {
		s.WithCaFile(f)
	}
}

type GrpcTunnelClient interface {
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

type GrpcTunnelClientImpl struct {
	ctx context.Context
	cfg *config

	// tunnel client

	// logger
	log logging.Logger
}

func New(opts ...Option) GrpcTunnelClientImpl {
	s := &GrpcTunnelClientImpl{
		cfg: &config{
			skipVerify: true,
			inSecure:   true,
		},
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *GrpcTunnelClientImpl) WithLogger(l logging.Logger) {
	s.log = l
}

func (s *GrpcTunnelClientImpl) WithAddress(a string) {
	s.cfg.address = a
}

func (s *GrpcTunnelClientImpl) WithCertFile(f string) {
	s.cfg.certFile = f
}

func (s *GrpcTunnelClientImpl) WithKeyFile(f string) {
	s.cfg.keyFile = f
}

func (s *GrpcTunnelClientImpl) WithCaFile(f string) {
	s.cfg.caFile = f
}

func (s *GrpcTunnelClientImpl) Start() error {
	errChannel := make(chan error)
	go func() {
		if err := s.run(); err != nil {
			errChannel <- errors.Wrap(err, errStartGRPCTunnelClient)
		}
		errChannel <- nil
	}()
	//fmt.Printf("Error start grpc tunnel server: %v\n", errChannel)
	return nil
}

func (s *GrpcTunnelClientImpl) run() error {
	return nil
}
