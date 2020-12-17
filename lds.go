package xds

import (
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/wrappers"

	accesslogv3 "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	alsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/grpc/v3"
	httpconnmgrv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	wellknownv3 "github.com/envoyproxy/go-control-plane/pkg/wellknown"
)

type LDSConfig struct {
	Listen    LDSListenConfig    `yaml:"listen"           validate:"required"`
	Server    LDSServerConfig    `yaml:"server"           validate:"required"`
	Timeout   LDSTimeoutConfig   `yaml:"timeout"          validate:"required"`
	AccessLog LDSAccessLogConfig `yaml:"accesslog"        validate:"required"`
}

type LDSListenConfig struct {
	Protocol string `yaml:"protocol"         validate:"required"`
	IP       string `yaml:"ip"               validate:"required,ip"`
	Port     uint32 `yaml:"port"             validate:"required,gte=1,lte=65535"`
}

func (c LDSListenConfig) Address() *corev3.Address {
	switch c.Protocol {
	case "tcp":
		return c.addr(corev3.SocketAddress_TCP)
	case "udp":
		return c.addr(corev3.SocketAddress_UDP)
	default:
		return c.addr(corev3.SocketAddress_TCP)
	}
}

func (c LDSListenConfig) addr(protocol corev3.SocketAddress_Protocol) *corev3.Address {
	return &corev3.Address{
		Address: &corev3.Address_SocketAddress{
			SocketAddress: &corev3.SocketAddress{
				Protocol: protocol,
				Address:  c.IP,
				PortSpecifier: &corev3.SocketAddress_PortValue{
					PortValue: c.Port,
				},
			},
		},
	}
}

type LDSServerConfig struct {
	ServerName     string `yaml:"name"             validate:"required,ascii"`
	UseRemoteAddr  bool   `yaml:"use-remote-addr"  validate:"required"`
	SkipXffAppend  bool   `yaml:"skip-xff-append"  validate:"required"`
	XffTrustedHops uint32 `yaml:"xff-trusted-hops" validate"required"`
}

type LDSTimeoutConfig struct {
	RequestTimeout uint32 `yaml:"request-timeout"  validate:"required,gte=1"`
	DrainTimeout   uint32 `yaml:"drain-timeout"    validate:"required,gte=1"`
	IdleTimeout    uint32 `yaml:"idle-timeout"     validate:"required,gte=1"`
	MaxDuration    uint32 `yaml:"max-duration"     validate:"required,gte=1"`
}

func (c LDSTimeoutConfig) RequestTimeoutSecond() time.Duration {
	return time.Duration(c.RequestTimeout) * time.Second
}

func (c LDSTimeoutConfig) DrainTimeoutSecond() time.Duration {
	return time.Duration(c.DrainTimeout) * time.Second
}

func (c LDSTimeoutConfig) IdleTimeoutSecond() time.Duration {
	return time.Duration(c.IdleTimeout) * time.Second
}

func (c LDSTimeoutConfig) MaxDurationSecond() time.Duration {
	return time.Duration(c.MaxDuration) * time.Second
}

type LDSAccessLogConfig struct {
	LogId         string `yaml:"log-id"          validate:"required"`
	FlushInterval uint32 `yaml:"flush-interval"  validate:"required,gte=1"`
	BufferSize    uint32 `yaml:"buffer-size"     validate:"required,gte=1"`
}

func (c LDSAccessLogConfig) FlushIntervalSecond() time.Duration {
	return time.Duration(c.FlushInterval) * time.Second
}

type ldsOptFunc func(*ldsOpt)

type ldsOpt struct {
	statPrefix string
}

func LdsStatPrefix(prefix string) ldsOptFunc {
	return func(opt *ldsOpt) {
		opt.statPrefix = prefix
	}
}

func initLdsOpt(opt *ldsOpt) {
	if len(opt.statPrefix) < 1 {
		opt.statPrefix = "ingress_http"
	}
}

type listenerDiscoveryService struct {
	opt       *ldsOpt
	xdsConfig *corev3.ConfigSource
	version   uint64
}

func newListenerDiscoveryService(xdsConfig *corev3.ConfigSource, funcs ...ldsOptFunc) *listenerDiscoveryService {
	opt := new(ldsOpt)
	for _, fn := range funcs {
		fn(opt)
	}
	initLdsOpt(opt)

	return &listenerDiscoveryService{
		opt:       opt,
		xdsConfig: xdsConfig,
		version:   uint64(0),
	}
}

func (s *listenerDiscoveryService) increVersion() uint64 {
	return atomic.AddUint64(&s.version, 1)
}

func (s *listenerDiscoveryService) httpConnectionManager(config LDSConfig) *httpconnmgrv3.HttpConnectionManager {
	return &httpconnmgrv3.HttpConnectionManager{
		CodecType:                 httpconnmgrv3.HttpConnectionManager_AUTO,
		StatPrefix:                s.opt.statPrefix,
		CommonHttpProtocolOptions: s.commonHttpProtocolOptions(config),
		UseRemoteAddress:          &wrappers.BoolValue{Value: config.Server.UseRemoteAddr},
		SkipXffAppend:             config.Server.SkipXffAppend,
		XffNumTrustedHops:         config.Server.XffTrustedHops,
		ServerName:                strings.Join([]string{config.Server.ServerName, Version}, "/"),
		RequestTimeout:            ptypes.DurationProto(config.Timeout.RequestTimeoutSecond()),
		DrainTimeout:              ptypes.DurationProto(config.Timeout.DrainTimeoutSecond()),
		RouteSpecifier:            s.routeSpecifier(),
		HttpFilters:               s.connHttpFilters(),
	}
}

func (s *listenerDiscoveryService) commonHttpProtocolOptions(config LDSConfig) *corev3.HttpProtocolOptions {
	// https://www.envoyproxy.io/docs/envoy/v1.15.0/api-v3/config/core/v3/protocol.proto#envoy-v3-api-msg-config-core-v3-httpprotocoloptions
	return &corev3.HttpProtocolOptions{
		IdleTimeout:           ptypes.DurationProto(config.Timeout.IdleTimeoutSecond()),
		MaxConnectionDuration: ptypes.DurationProto(config.Timeout.MaxDurationSecond()),
	}
}

func (s *listenerDiscoveryService) routeSpecifier() *httpconnmgrv3.HttpConnectionManager_Rds {
	// ref: rds.routeConfiguration
	routeConfigName := xdsName("example-xds-route-config")
	return &httpconnmgrv3.HttpConnectionManager_Rds{
		Rds: &httpconnmgrv3.Rds{
			RouteConfigName: routeConfigName,
			ConfigSource:    s.xdsConfig,
		},
	}
}

func (s *listenerDiscoveryService) connHttpFilters() []*httpconnmgrv3.HttpFilter {
	// https://www.envoyproxy.io/docs/envoy/v1.15.0/api-v3/extensions/filters/network/http_connection_manager/v3/http_connection_manager.proto#envoy-v3-api-msg-extensions-filters-network-http-connection-manager-v3-httpfilter
	return []*httpconnmgrv3.HttpFilter{
		// declare as name as "http.router"
		&httpconnmgrv3.HttpFilter{
			Name: wellknownv3.Router,
		},
	}
}

func (s *listenerDiscoveryService) connHttpAccesslog(alsConfig *any.Any) []*accesslogv3.AccessLog {
	// https://www.envoyproxy.io/docs/envoy/v1.15.0/api-v3/config/accesslog/v3/accesslog.proto#config-accesslog-v3-accesslog
	return []*accesslogv3.AccessLog{
		&accesslogv3.AccessLog{
			Name: wellknownv3.HTTPGRPCAccessLog,
			ConfigType: &accesslogv3.AccessLog_TypedConfig{
				TypedConfig: alsConfig,
			},
		},
	}
}

func (s *listenerDiscoveryService) listenerFilters(managerConfig *any.Any) []*listenerv3.Filter {
	// https://www.envoyproxy.io/docs/envoy/v1.15.0/api-v3/extensions/filters/network/http_connection_manager/v3/http_connection_manager.proto
	return []*listenerv3.Filter{
		&listenerv3.Filter{
			// declare as name as "network.http_connection_manager"
			Name:       wellknownv3.HTTPConnectionManager,
			ConfigType: &listenerv3.Filter_TypedConfig{TypedConfig: managerConfig},
		},
	}
}

func (s *listenerDiscoveryService) listener(filters []*listenerv3.Filter, config LDSListenConfig) *listenerv3.Listener {
	listenerName := xdsName("example-xds-listener")
	return &listenerv3.Listener{
		Name:    listenerName,
		Address: config.Address(),
		FilterChains: []*listenerv3.FilterChain{
			&listenerv3.FilterChain{Filters: filters},
		},
	}
}

func (s *listenerDiscoveryService) accesslogConfig(config LDSAccessLogConfig) *alsv3.HttpGrpcAccessLogConfig {
	// https://www.envoyproxy.io/docs/envoy/v1.15.0/api-v3/extensions/access_loggers/grpc/v3/als.proto#envoy-v3-api-msg-extensions-access-loggers-grpc-v3-commongrpcaccesslogconfig
	return &alsv3.HttpGrpcAccessLogConfig{
		CommonConfig: &alsv3.CommonGrpcAccessLogConfig{
			LogName:             config.LogId,
			TransportApiVersion: resourcev3.DefaultAPIVersion,
			BufferFlushInterval: ptypes.DurationProto(config.FlushIntervalSecond()),
			BufferSizeBytes:     &wrappers.UInt32Value{Value: config.BufferSize},
			GrpcService:         alsGRPCService(),
		},
	}
}

func (s *listenerDiscoveryService) create(config LDSConfig) (string, *listenerv3.Listener, error) {
	// https://github.com/envoyproxy/go-control-plane/blob/000e06b258c1cf20548035ce4443780f8ccbd903/pkg/test/resource/v2/resource.go#L173
	acclogConfig := s.accesslogConfig(config.AccessLog)
	alsConfig, err := ptypes.MarshalAny(acclogConfig)
	if err != nil {
		return "", nil, err
	}

	manager := s.httpConnectionManager(config)
	manager.AccessLog = s.connHttpAccesslog(alsConfig)

	managerConfig, err := ptypes.MarshalAny(manager)
	if err != nil {
		return "", nil, err
	}

	filters := s.listenerFilters(managerConfig)
	version := strconv.FormatUint(s.increVersion(), 10)
	return version, s.listener(filters, config.Listen), nil
}
