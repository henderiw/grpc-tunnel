package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/openconfig/grpctunnel/bidi"
	tpb "github.com/openconfig/grpctunnel/proto/tunnel"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/pkg/errors"
	"github.com/yndd/ndd-runtime/pkg/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	errStartGRPCTunnelClient = "cannot start grpc tunnel client"

	// for setting retry backoff when waiting for target.
	retryBaseDelay     = time.Second
	retryMaxDelay      = time.Minute
	retryRandomization = 0.5
)

// Option can be used to manipulate Options.
type Option func(GrpcTunnelClient)

func WithLogger(l logging.Logger) Option {
	return func(s GrpcTunnelClient) {
		s.WithLogger(l)
	}
}

func WithTunnelServerAddress(f string) Option {
	return func(s GrpcTunnelClient) {
		s.WithTunnelServerAddress(f)
	}
}

func WithDialTarget(t, tt string) Option {
	return func(s GrpcTunnelClient) {
		s.WithDialTarget(t, tt)
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
	WithTunnelServerAddress(string)
	WithDialTarget(string, string)
	WithCertFile(string)
	WithKeyFile(string)
	WithCaFile(string)
	Start() error
}

type config struct {
	tunnelServerAddress string
	dialTarget          string
	dialTargetType      string
	certFile            string
	keyFile             string
	caFile              string
}

type GrpcTunnelClientImpl struct {
	ctx context.Context
	cfg *config

	// peers
	peerMux sync.Mutex
	peers   map[tunnel.Target]struct{}

	// targets
	targetMux sync.Mutex
	targets   map[tunnel.Target]struct{}

	// logger
	log logging.Logger
}

func New(opts ...Option) GrpcTunnelClient {
	x := &GrpcTunnelClientImpl{
		cfg: &config{},
		ctx: context.Background(),

		peers:   make(map[tunnel.Target]struct{}),
		targets: make(map[tunnel.Target]struct{}),
	}

	for _, opt := range opts {
		opt(x)
	}
	return x
}

func (x *GrpcTunnelClientImpl) WithLogger(l logging.Logger) {
	x.log = l
}

func (x *GrpcTunnelClientImpl) WithTunnelServerAddress(a string) {
	x.cfg.tunnelServerAddress = a
}

func (x *GrpcTunnelClientImpl) WithDialTarget(t, tt string) {
	x.cfg.dialTarget = t
	x.cfg.dialTargetType = tt
}

func (x *GrpcTunnelClientImpl) WithCertFile(f string) {
	x.cfg.certFile = f
}

func (x *GrpcTunnelClientImpl) WithKeyFile(f string) {
	x.cfg.keyFile = f
}

func (x *GrpcTunnelClientImpl) WithCaFile(f string) {
	x.cfg.caFile = f
}

func (x *GrpcTunnelClientImpl) Start() error {
	errChannel := make(chan error)
	go func() {
		if err := x.run(x.ctx); err != nil {
			errChannel <- errors.Wrap(err, errStartGRPCTunnelClient)
		}
		errChannel <- nil
	}()
	return nil
}

func (x *GrpcTunnelClientImpl) run(ctx context.Context) error {
	log := x.log.WithValues("tunnel server address", x.cfg.tunnelServerAddress)
	log.Debug("grpctunnel client run...")

	// TODO add better certificate handling, right now self signed certificate
	opts := x.getGrpcOptions()

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
			conn, err = grpc.DialContext(gnmiCtx, x.cfg.tunnelServerAddress, opts...)
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
		PeerAddHandler: x.peerAddHandler(),
		PeerDelHandler: x.peerDelHandler(),
		Subscriptions:  []string{x.cfg.dialTargetType},
	}, x.targets)
	if err != nil {
		log.Debug("failed to create tunnel client", "error", err)
		return err
	}

	// register and start client
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, 2)
	go func() {
		if err := client.Register(ctx); err != nil {
			errCh <- err
			log.Debug("registration failed", "error", err)
			return
		}
		client.Start(ctx)
		if err := client.Error(); err != nil {
			log.Debug("client start failed", "error", err)
			errCh <- err
		}
	}()

	dialTarget := tunnel.Target{
		ID:   x.cfg.dialTarget,
		Type: x.cfg.dialTargetType,
	}
	foundDialTarget := func() bool {
		x.peerMux.Lock()
		defer x.peerMux.Unlock()
		_, ok := x.peers[dialTarget]
		return ok
	}

	// Dial the target with retry.
	go func() {
		bo := getBackOff()
		for !foundDialTarget() {
			wait := bo.NextBackOff()
			log.Debug("dial target not found", "dialTarget", dialTarget.ID, "type", dialTarget.Type, "wait", wait)
			time.Sleep(wait)
		}
		session, err := client.NewSession(dialTarget)
		if err != nil {
			log.Debug("new session failed", "error", err)
			errCh <- err
			return
		}
		log.Debug("new session established", "dialTarget", dialTarget)

		// Once a tunnel session is established, it connects it to a stdio.
		stdio := &stdIOConn{Reader: os.Stdin, WriteCloser: os.Stdout}
		if err = bidi.Copy(session, stdio); err != nil {
			log.Debug("bidi copy failed", "error", err)
			return
		}

	}()

	// Listen for any request to create a new session.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return fmt.Errorf("exiting: %s", err)
	}
}

func (x *GrpcTunnelClientImpl) peerAddHandler() func(t tunnel.Target) error {
	return func(t tunnel.Target) error {
		x.peerMux.Lock()
		defer x.peerMux.Unlock()
		x.peers[t] = struct{}{}
		x.log.Debug("peer added", "tunnel target", t)
		return nil
	}
}

func (x *GrpcTunnelClientImpl) peerDelHandler() func(t tunnel.Target) error {
	return func(t tunnel.Target) error {
		x.peerMux.Lock()
		defer x.peerMux.Unlock()
		if _, ok := x.peers[t]; ok {
			delete(x.peers, t)
			x.log.Debug("peer deleted", "tunnel target", t)
		} else {
			x.log.Debug("peer delete but did not exists", "tunnel target", t)
		}
		return nil
	}
}

func (x *GrpcTunnelClientImpl) getGrpcOptions() []grpc.DialOption {
	opts := []grpc.DialOption{
		grpc.WithBlock(),
	}
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}
	opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	return opts
}

func getBackOff() *backoff.ExponentialBackOff {
	bo := backoff.NewExponentialBackOff()
	bo.MaxElapsedTime = 0 // Retry Subscribe indefinitely.
	bo.InitialInterval = retryBaseDelay
	bo.MaxInterval = retryMaxDelay
	bo.RandomizationFactor = retryRandomization
	return bo
}

type stdIOConn struct {
	io.Reader
	io.WriteCloser
}
