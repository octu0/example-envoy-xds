package xds

import (
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
)

const (
	BootstrapXdsClusterName string        = "xds_cluster"
	BootstrapAlsClusterName string        = "als_cluster"
	EnvoyRESTRefreshDelay   time.Duration = 10 * time.Second
	EnvoyRESTRequestTimeout time.Duration = 10 * time.Second
	EnvoyGRPCRequestTimeout time.Duration = 10 * time.Second
)

func xdsName(values ...string) string {
	return strings.ReplaceAll(strings.Join(values, "_"), "-", "_")
}

func xdsConfigSource() *corev3.ConfigSource {
	return &corev3.ConfigSource{
		ResourceApiVersion: resourcev3.DefaultAPIVersion,
		ConfigSourceSpecifier: &corev3.ConfigSource_ApiConfigSource{
			// https://www.envoyproxy.io/docs/envoy/v1.15.0/api-v3/config/core/v3/config_source.proto.html
			ApiConfigSource: &corev3.ApiConfigSource{
				TransportApiVersion:       resourcev3.DefaultAPIVersion,
				ApiType:                   corev3.ApiConfigSource_GRPC,
				SetNodeOnFirstMessageOnly: true,
				RefreshDelay:              ptypes.DurationProto(EnvoyRESTRefreshDelay),
				RequestTimeout:            ptypes.DurationProto(EnvoyRESTRequestTimeout),
				GrpcServices:              xdsGRPCServices(),
			},
		},
	}
}

func xdsGRPCServices() []*corev3.GrpcService {
	// https://www.envoyproxy.io/docs/envoy/v1.15.0/api-v3/config/core/v3/grpc_service.proto#envoy-v3-api-msg-config-core-v3-grpcservice
	return []*corev3.GrpcService{
		grpcService(BootstrapXdsClusterName),
	}
}

func alsGRPCService() *corev3.GrpcService {
	return grpcService(BootstrapAlsClusterName)
}

func grpcService(clusterName string) *corev3.GrpcService {
	return &corev3.GrpcService{
		Timeout: ptypes.DurationProto(EnvoyGRPCRequestTimeout),
		TargetSpecifier: &corev3.GrpcService_EnvoyGrpc_{
			EnvoyGrpc: &corev3.GrpcService_EnvoyGrpc{ClusterName: clusterName},
		},
	}
}
