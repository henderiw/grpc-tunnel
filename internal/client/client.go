package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/openconfig/grpctunnel/bidi"
	tpb "github.com/openconfig/grpctunnel/proto/tunnel"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/pkg/errors"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	errStartGRPCTunnelClient = "cannot start grpc tunnel client"

	googledest        = "google-tunnel-server"
	googledestAddress = "34.79.210.188:57401"
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

type tunnelConfig struct {
	target      map[string]*target
	destination map[string]*destination
}

type destination struct {
	address    string // address of the grpc tunnel server + port
	inSecure   bool
	skipVerify bool
	caFile     string
	certFile   string
	keyFile    string
}

type target struct {
	ID   string
	Type string
}

type tunnelDestinationClient struct {
	// gRPC connection towards the tunnel server
	conn *grpc.ClientConn
	// the gRPC tunnel client
	client *tunnel.Client
}

type GrpcTunnelClientImpl struct {
	ctx       context.Context
	tunnelCfg *tunnelConfig

	// tunnel client
	m             sync.Mutex
	tunnelClients map[string]*tunnelDestinationClient

	// logger
	log logging.Logger
}

func New(opts ...Option) GrpcTunnelClient {
	s := &GrpcTunnelClientImpl{
		tunnelCfg: &tunnelConfig{
			target: map[string]*target{
				"ssh": {
					ID:   "testID1",
					Type: "SSH",
				},
			},
			destination: map[string]*destination{
				googledest: {
					address:  googledestAddress,
					inSecure: false,
				},
			},
		},
		tunnelClients: make(map[string]*tunnelDestinationClient),
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
	//s.cfg.address = a
}

func (s *GrpcTunnelClientImpl) WithCertFile(f string) {
	//s.cfg.certFile = f
}

func (s *GrpcTunnelClientImpl) WithKeyFile(f string) {
	//s.cfg.keyFile = f
}

func (s *GrpcTunnelClientImpl) WithCaFile(f string) {
	//s.cfg.caFile = f
}

func (s *GrpcTunnelClientImpl) Start() error {
	errChannel := make(chan error)
	go func() {
		if err := s.run(); err != nil {
			errChannel <- errors.Wrap(err, errStartGRPCTunnelClient)
		}
		errChannel <- nil
	}()
	return nil
}

func (s *GrpcTunnelClientImpl) run() error {
	log := s.log.WithValues("destination address", s.tunnelCfg.destination[googledest].address)
	log.Debug("run...")
	opts := []grpc.DialOption{
		// grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	}
	if s.tunnelCfg.destination[googledest].inSecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	}

	log.Debug("client setup ...")
	ctx := context.Background()

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
			conn, err = grpc.DialContext(gnmiCtx, s.tunnelCfg.destination[googledest].address, opts...)
			if err != nil {
				log.Debug("tunnel failed to connect", "error", err)
				time.Sleep(5 * time.Second)
				continue
			}
		}
		break
	}
	defer conn.Close()

	client, err := tunnel.NewClient(tpb.NewTunnelClient(conn), tunnel.ClientConfig{
		RegisterHandler: func(t tunnel.Target) error { return nil },
		Handler:         s.tunnelHandlerFunc(googledest),
	}, nil)
	if err != nil {
		s.log.Debug("failed to create tunnel client", "error", err)
		return err
	}
	log.Debug("tunnel destination successfull")
	// Register and start listening.
	err = client.Register(ctx)
	if err != nil {
		s.log.Debug("tunnel destination registration failed", "error", err)
		return err
	}
	log.Debug("tunnel client registered")
	s.m.Lock()
	defer s.m.Unlock()
	s.tunnelClients[googledest] = &tunnelDestinationClient{
		conn:   conn,
		client: client,
	}

	// create targets
	for _, t := range s.tunnelCfg.target {
		log.Debug("target handler", "ID", t.ID, "Type", t.Type)
		s.startTunnelHandlerDestination(t)
	}

	// blocking call
	client.Start(ctx)
	//

	return client.Error()
}

func (s *GrpcTunnelClientImpl) tunnelHandlerFunc(dn string) func(t tunnel.Target, i io.ReadWriteCloser) error {
	return func(t tunnel.Target, i io.ReadWriteCloser) error {
		dialAddr := s.tunnelCfg.destination[dn].address
		conn, err := net.Dial("tcp", dialAddr)
		if err != nil {
			return fmt.Errorf("failed to dial %s: %v", dialAddr, err)
		}
		// start bidirectional copy
		if err = bidi.Copy(i, conn); err != nil {
			return fmt.Errorf("bidi copy error: %v", err)
		}
		return nil
	}
}

func (s *GrpcTunnelClientImpl) startTunnelHandlerDestination(t *target) {
	s.m.Lock()
	defer s.m.Unlock()
	tc, ok := s.tunnelClients[googledest]
	if !ok {
		s.log.Debug("failed to create target")
		return
	}

	if err := tc.client.NewTarget(tunnel.Target{ID: t.ID, Type: t.Type}); err != nil {
		s.log.Debug("failed to register target", "ID", t.ID, "Type", t.Type)
		return
	}
	s.log.Debug("registered target", "ID", t.ID, "Type", t.Type)
}
