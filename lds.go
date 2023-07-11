package xds

import (
	"strconv"
	"strings"
	"sync/atomic"

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
