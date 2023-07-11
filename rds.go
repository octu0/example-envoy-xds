package xds

import (
	"strconv"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/wrappers"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
)

const (
	defaultRetryBackOffIntervalBase time.Duration = 100 * time.Millisecond
	defaultRetryBackOffIntervalMax  time.Duration = 3 * time.Second
	defaultRetryPerTryTimeout       time.Duration = 1 * time.Second
)

type rdsOptFunc func(*rdsOpt)

type rdsOpt struct {
	retryBackOffIntervalBase   time.Duration
	retryBackOffIntervalMax    time.Duration
	retryPerTryTimeout         time.Duration
	retryHostSelectionAttempts int64
}

func RdsRetryBackOffIntervalBase(dur time.Duration) rdsOptFunc {
	return func(opt *rdsOpt) {
		opt.retryBackOffIntervalBase = dur
	}
}

func RdsRetryBackOffIntervalMax(dur time.Duration) rdsOptFunc {
	return func(opt *rdsOpt) {
		opt.retryBackOffIntervalMax = dur
	}
}

func RdsRetryPerTryTimeout(dur time.Duration) rdsOptFunc {
	return func(opt *rdsOpt) {
		opt.retryPerTryTimeout = dur
	}
}

func RdsRetryHostSelectionAttempts(times int64) rdsOptFunc {
	return func(opt *rdsOpt) {
		opt.retryHostSelectionAttempts = times
	}
}

func initRdsOpt(opt *rdsOpt) {
	if opt.retryBackOffIntervalBase < 1 {
		opt.retryBackOffIntervalBase = defaultRetryBackOffIntervalBase
	}
	if opt.retryBackOffIntervalMax < 1 {
		opt.retryBackOffIntervalMax = defaultRetryBackOffIntervalMax
	}
	if opt.retryPerTryTimeout < 1 {
		opt.retryPerTryTimeout = defaultRetryPerTryTimeout
	}
}

type routeDiscoveryService struct {
	opt       *rdsOpt
	xdsConfig *corev3.ConfigSource
	version   uint64
}

func (r *routeDiscoveryService) increVersion() uint64 {
	return atomic.AddUint64(&r.version, 1)
}

func (r *routeDiscoveryService) retryBackOff() *routev3.RetryPolicy_RetryBackOff {
	return &routev3.RetryPolicy_RetryBackOff{
		BaseInterval: ptypes.DurationProto(r.opt.retryBackOffIntervalBase),
		MaxInterval:  ptypes.DurationProto(r.opt.retryBackOffIntervalMax),
	}
}

func (r *routeDiscoveryService) retryPolicyRetryOn(retryCount uint32) *routev3.RetryPolicy {
	return &routev3.RetryPolicy{
		RetryOn:       "5xx,gateway-error,reset,connect-failure",
		NumRetries:    &wrappers.UInt32Value{Value: retryCount},
		PerTryTimeout: ptypes.DurationProto(r.opt.retryPerTryTimeout),
		RetryBackOff:  r.retryBackOff(),
	}
}

func (r *routeDiscoveryService) retryPolicyNoRetry() *routev3.RetryPolicy {
	return &routev3.RetryPolicy{
		RetryOn:    "",
		NumRetries: &wrappers.UInt32Value{Value: 0},
	}
}

func (r *routeDiscoveryService) retryPolicy(action RDSActionConfig) *routev3.RetryPolicy {
	switch action.RetryPolicy {
	case "off", "no":
		return r.retryPolicyNoRetry()
	case "retry1":
		return r.retryPolicyRetryOn(1)
	case "retry5":
		return r.retryPolicyRetryOn(5)
	default:
		return r.retryPolicyNoRetry()
	}
}

func (r *routeDiscoveryService) weightedClusters(totalWeight uint32, clusters []*routev3.WeightedCluster_ClusterWeight) *routev3.RouteAction_WeightedClusters {
	return &routev3.RouteAction_WeightedClusters{
		WeightedClusters: &routev3.WeightedCluster{
			Clusters:    clusters,
			TotalWeight: &wrappers.UInt32Value{Value: totalWeight},
		},
	}
}

func (r *routeDiscoveryService) cluster(target RDSClusterWeightConfig) *routev3.WeightedCluster_ClusterWeight {
	// ref: cds.clusterConfig
	clusterName := xdsName("example-xds-cluster", target.ClusterName)
	return &routev3.WeightedCluster_ClusterWeight{
		Name:   clusterName,
		Weight: &wrappers.UInt32Value{Value: target.Weight},
	}
}

func (r *routeDiscoveryService) clusters(targets []RDSClusterWeightConfig) []*routev3.WeightedCluster_ClusterWeight {
	clusters := make([]*routev3.WeightedCluster_ClusterWeight, len(targets))
	for idx, t := range targets {
		clusters[idx] = r.cluster(t)
	}
	return clusters
}

func (r *routeDiscoveryService) clusterTotalWeight(targets []RDSClusterWeightConfig) uint32 {
	weight := uint32(0)
	for _, t := range targets {
		weight += t.Weight
	}
	return weight
}

func (r *routeDiscoveryService) header(h RDSClusterHeaderConfig) *routev3.HeaderMatcher {
	return &routev3.HeaderMatcher{
		Name: h.HeaderName,
		HeaderMatchSpecifier: &routev3.HeaderMatcher_StringMatch{
			StringMatch: &matcherv3.StringMatcher{
				MatchPattern: &matcherv3.StringMatcher_Exact{
					Exact: h.StringMatch.Exact,
				},
			},
		},
	}
}

func (r *routeDiscoveryService) clusterHeaders(cluster RDSClusterConfig) []*routev3.HeaderMatcher {
	headers := make([]*routev3.HeaderMatcher, len(cluster.Headers))
	for i, h := range cluster.Headers {
		headers[i] = r.header(h)
	}
	return headers
}

func (r *routeDiscoveryService) route(cluster RDSClusterConfig, action RDSActionConfig) *routev3.Route {
	clusters := r.clusters(cluster.Target)
	totalWeights := r.clusterTotalWeight(cluster.Target)

	routeName := xdsName("example-xds-route", cluster.Prefix)
	return &routev3.Route{
		Name: routeName,
		Match: &routev3.RouteMatch{
			PathSpecifier: &routev3.RouteMatch_Prefix{Prefix: cluster.Prefix},
			Headers:       r.clusterHeaders(cluster),
		},
		// https://github.com/envoyproxy/go-control-plane/blob/d5e54b318e480a7dcc1cadf1a4406145669a5965/envoy/config/route/v3/route_components.pb.go#L1379
		Action: &routev3.Route_Route{
			Route: &routev3.RouteAction{
				ClusterSpecifier: r.weightedClusters(totalWeights, clusters),
				RetryPolicy:      r.retryPolicy(action),
				Timeout:          ptypes.DurationProto(action.TimeoutSecond()),
				IdleTimeout:      ptypes.DurationProto(action.IdleTimeoutSecond()),
			},
		},
	}
}

func (r *routeDiscoveryService) virtualRoutes(clusters []RDSClusterConfig, action RDSActionConfig) []*routev3.Route {
	routes := make([]*routev3.Route, len(clusters))
	for i, cluster := range clusters {
		routes[i] = r.route(cluster, action)
	}
	return routes
}

func (r *routeDiscoveryService) virtualHosts(configs []RDSConfig) []*routev3.VirtualHost {
	vhosts := make([]*routev3.VirtualHost, len(configs))
	for idx, config := range configs {
		vhostName := xdsName("example-xds-vhost", config.VHostName)
		vhosts[idx] = &routev3.VirtualHost{
			Name:    vhostName,
			Domains: config.Domain,
			Routes:  r.virtualRoutes(config.Cluster, config.Action),
		}
	}
	return vhosts
}

func (r *routeDiscoveryService) routeConfiguration(configs []RDSConfig) *routev3.RouteConfiguration {
	// ref: lds.routeSpecifier
	routeConfigName := xdsName("example-xds-route-config")
	return &routev3.RouteConfiguration{
		Name:         routeConfigName,
		VirtualHosts: r.virtualHosts(configs),
	}
}

func (r *routeDiscoveryService) create(configs []RDSConfig) (string, *routev3.RouteConfiguration, error) {
	version := strconv.FormatUint(r.increVersion(), 10)
	return version, r.routeConfiguration(configs), nil
}

func newRouteDiscoveryService(xdsConfig *corev3.ConfigSource, funcs ...rdsOptFunc) *routeDiscoveryService {
	opt := new(rdsOpt)
	for _, fn := range funcs {
		fn(opt)
	}
	initRdsOpt(opt)

	return &routeDiscoveryService{
		opt:       opt,
		xdsConfig: xdsConfig,
		version:   uint64(0),
	}
}
