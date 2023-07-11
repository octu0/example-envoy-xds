package xds

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"

	alsv3 "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	clusterservicev3 "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	endpointservicev3 "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservicev3 "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservicev3 "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	serverv3 "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	// to be
	discoverygrpcv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	runtimeservicev3 "github.com/envoyproxy/go-control-plane/envoy/service/runtime/v3"
	secretservicev3 "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
)

const (
	defaultXdsListenAddr         string = "[0.0.0.0]:8000"
	defaultAlsListenAddr         string = "[0.0.0.0]:8001"
	defaultGrpcConcurrentStreams uint32 = 1000000
)

type serverOptFunc func(*serverOpt)

type serverOpt struct {
	xdsListenAddr        string
	alsListenAddr        string
	maxConcurrentStreams uint32
}

func XdsListenAddr(addr string) serverOptFunc {
	return func(opt *serverOpt) {
		opt.xdsListenAddr = addr
	}
}

func AlsListenAddr(addr string) serverOptFunc {
	return func(opt *serverOpt) {
		opt.alsListenAddr = addr
	}
}

func MaxConcurrentStreams(n uint32) serverOptFunc {
	return func(opt *serverOpt) {
		opt.maxConcurrentStreams = n
	}
}

func initOpt(opt *serverOpt) {
	if len(opt.xdsListenAddr) < 1 {
		opt.xdsListenAddr = defaultXdsListenAddr
	}
	if len(opt.alsListenAddr) < 1 {
		opt.alsListenAddr = defaultAlsListenAddr
	}
	if opt.maxConcurrentStreams < 1 {
		opt.maxConcurrentStreams = defaultGrpcConcurrentStreams
	}
}

type server struct {
	opt        *serverOpt
	xdsSvr     *grpc.Server
	alsSvr     *grpc.Server
	xdsHandler serverv3.Server
	alsHandler *accesslogServiceHandler
}

func (s *server) registerXdsService() {
	clusterservicev3.RegisterClusterDiscoveryServiceServer(s.xdsSvr, s.xdsHandler)
	endpointservicev3.RegisterEndpointDiscoveryServiceServer(s.xdsSvr, s.xdsHandler)
	listenerservicev3.RegisterListenerDiscoveryServiceServer(s.xdsSvr, s.xdsHandler)
	routeservicev3.RegisterRouteDiscoveryServiceServer(s.xdsSvr, s.xdsHandler)
	// to be
	discoverygrpcv3.RegisterAggregatedDiscoveryServiceServer(s.xdsSvr, s.xdsHandler)
	runtimeservicev3.RegisterRuntimeDiscoveryServiceServer(s.xdsSvr, s.xdsHandler)
	secretservicev3.RegisterSecretDiscoveryServiceServer(s.xdsSvr, s.xdsHandler)
}

func (s *server) registerAlsService() {
	// https://godoc.org/github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3#AccessLogServiceServer
	alsv3.RegisterAccessLogServiceServer(s.alsSvr, s.alsHandler)
}

func (s *server) listenXds() (net.Listener, error) {
	log.Printf("info: xds server listen: %s", s.opt.xdsListenAddr)
	listener, err := net.Listen("tcp", s.opt.xdsListenAddr)
	if err != nil {
		log.Printf("error: addr '%s' listen error: %s", s.opt.xdsListenAddr, err.Error())
		return nil, err
	}
	return listener, nil
}

func (s *server) listenAls() (net.Listener, error) {
	log.Printf("info: als server listen: %s", s.opt.alsListenAddr)
	listener, err := net.Listen("tcp", s.opt.alsListenAddr)
	if err != nil {
		log.Printf("error: addr '%s' listen error: %s", s.opt.alsListenAddr, err.Error())
		return nil, err
	}
	return listener, nil
}

func (s *server) Start() error {
	xdsListen, err := s.listenXds()
	if err != nil {
		return err
	}
	alsListen, err := s.listenAls()
	if err != nil {
		return err
	}

	s.registerXdsService()
	s.registerAlsService()

	errors := make(chan error, 0)
	go func() {
		if err := s.xdsSvr.Serve(xdsListen); err != nil {
			log.Printf("error: xds serve error: %s", err.Error())
			errors <- err
		}
	}()
	go func() {
		if err := s.alsSvr.Serve(alsListen); err != nil {
			log.Printf("error: als serve error: %s", err.Error())
			errors <- err
		}
	}()
	return <-errors
}

func (s *server) Stop() error {
	log.Printf("info: stop grpc server")

	s.xdsSvr.Stop()
	s.alsSvr.Stop()

	return nil
}

func NewServer(ctx context.Context, cache cachev3.Cache, funcs ...serverOptFunc) *server {
	opt := new(serverOpt)
	for _, fn := range funcs {
		fn(opt)
	}
	initOpt(opt)

	xdsSvr := grpc.NewServer(
		grpc.MaxConcurrentStreams(opt.maxConcurrentStreams),
	)
	alsSvr := grpc.NewServer(
		grpc.MaxConcurrentStreams(opt.maxConcurrentStreams),
	)
	return &server{
		opt:        opt,
		xdsSvr:     xdsSvr,
		alsSvr:     alsSvr,
		xdsHandler: serverv3.NewServer(ctx, cache, nil),
		alsHandler: newAccesslogServiceHandler(newLoggerAccessLog()),
	}
}
