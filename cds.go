package xds

import (
	"log"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/wrappers"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
)

const (
	defaultClusterConnectionTimeout                time.Duration = 10 * time.Second
	defaultClusterUpstreamKeepAliveIntervalSeconds uint32        = 60
	defaultClusterUpstreamKeepAliveTimeSeconds     uint32        = 60
	defaultClusterRefreshIntervalBase              time.Duration = 5 * time.Second
	defaultClusterRefreshIntervalMax               time.Duration = 10 * time.Second
	defaultHealthCheckInitialJitter                time.Duration = 1 * time.Second
	defaultHealthCheckInitialIntervalOnAddCluster  time.Duration = 5 * time.Second
	defaultOutlierConsecutiveCount5xx              uint32        = 10
	defaultOutlierConsecutiveGatewayFailure        uint32        = 30
	defaultOutlierInterval                         time.Duration = 10 * time.Second
	defaultOutlierBaseEjectionTime                 time.Duration = 30 * time.Second
	defaultOutlierSuccessRateMinHosts              uint32        = 5
)

type cdsOptFunc func(*cdsOpt)

type cdsOpt struct {
	clusterConnectionTimeout                time.Duration
	clusterUpstreamKeepAliveIntervalSeconds uint32
	clusterUpstreamKeepAliveTimeSeconds     uint32
	clusterRefreshIntervalBase              time.Duration
	clusterRefreshIntervalMax               time.Duration
	healthCheckInitialJitter                time.Duration
	healthCheckInitialIntervalOnAddCluster  time.Duration
	outlierConsecutiveCount5xx              uint32
	outlierConsecutiveGatewayFailure        uint32
	outlierInterval                         time.Duration
	outlierBaseEjectionTime                 time.Duration
	outlierSuccessRateMinHosts              uint32
}

func CdsClusterConnectionTimeout(dur time.Duration) cdsOptFunc {
	return func(opt *cdsOpt) {
		opt.clusterConnectionTimeout = dur
	}
}

func CdsClusterUpstreamKeepAliveIntervalSeconds(seconds uint32) cdsOptFunc {
	return func(opt *cdsOpt) {
		opt.clusterUpstreamKeepAliveIntervalSeconds = seconds
	}
}

func CdsClusterUpstreamKeepAliveTimeSeconds(seconds uint32) cdsOptFunc {
	return func(opt *cdsOpt) {
		opt.clusterUpstreamKeepAliveTimeSeconds = seconds
	}
}

func CdsClusterRefreshIntervalBase(dur time.Duration) cdsOptFunc {
	return func(opt *cdsOpt) {
		opt.clusterRefreshIntervalBase = dur
	}
}

func CdsClusterRefreshIntervalMax(dur time.Duration) cdsOptFunc {
	return func(opt *cdsOpt) {
		opt.clusterRefreshIntervalMax = dur
	}
}

func CdsHealthCheckInitialJitter(dur time.Duration) cdsOptFunc {
	return func(opt *cdsOpt) {
		opt.healthCheckInitialJitter = dur
	}
}

func CdsHealthCheckInitialIntervalOnAddCluster(dur time.Duration) cdsOptFunc {
	return func(opt *cdsOpt) {
		opt.healthCheckInitialIntervalOnAddCluster = dur
	}
}

func CdsOutlierConsecutiveCount5xx(times uint32) cdsOptFunc {
	return func(opt *cdsOpt) {
		opt.outlierConsecutiveCount5xx = times
	}
}

func CdsOutlierConsecutiveGatewayFailure(times uint32) cdsOptFunc {
	return func(opt *cdsOpt) {
		opt.outlierConsecutiveGatewayFailure = times
	}
}

func CdsOutlierInterval(dur time.Duration) cdsOptFunc {
	return func(opt *cdsOpt) {
		opt.outlierInterval = dur
	}
}

func CdsOutlierBaseEjectionTime(dur time.Duration) cdsOptFunc {
	return func(opt *cdsOpt) {
		opt.outlierBaseEjectionTime = dur
	}
}

func CdsOutlierSuccessRateMinHosts(size uint32) cdsOptFunc {
	return func(opt *cdsOpt) {
		opt.outlierSuccessRateMinHosts = size
	}
}

func initCdsOpt(opt *cdsOpt) {
	if opt.clusterConnectionTimeout < 1 {
		opt.clusterConnectionTimeout = defaultClusterConnectionTimeout
	}
	if opt.clusterUpstreamKeepAliveIntervalSeconds < 1 {
		opt.clusterUpstreamKeepAliveIntervalSeconds = defaultClusterUpstreamKeepAliveIntervalSeconds
	}
	if opt.clusterUpstreamKeepAliveTimeSeconds < 1 {
		opt.clusterUpstreamKeepAliveTimeSeconds = defaultClusterUpstreamKeepAliveTimeSeconds
	}
	if opt.clusterRefreshIntervalBase < 1 {
		opt.clusterRefreshIntervalBase = defaultClusterRefreshIntervalBase
	}
	if opt.clusterRefreshIntervalMax < 1 {
		opt.clusterRefreshIntervalMax = defaultClusterRefreshIntervalMax
	}
	if opt.healthCheckInitialJitter < 1 {
		opt.healthCheckInitialJitter = defaultHealthCheckInitialJitter
	}
	if opt.healthCheckInitialIntervalOnAddCluster < 1 {
		opt.healthCheckInitialIntervalOnAddCluster = defaultHealthCheckInitialIntervalOnAddCluster
	}
	if opt.outlierConsecutiveCount5xx < 1 {
		opt.outlierConsecutiveCount5xx = defaultOutlierConsecutiveCount5xx
	}
	if opt.outlierConsecutiveGatewayFailure < 1 {
		opt.outlierConsecutiveGatewayFailure = defaultOutlierConsecutiveGatewayFailure
	}
	if opt.outlierInterval < 1 {
		opt.outlierInterval = defaultOutlierInterval
	}
	if opt.outlierBaseEjectionTime < 1 {
		opt.outlierBaseEjectionTime = defaultOutlierBaseEjectionTime
	}
	if opt.outlierSuccessRateMinHosts < 1 {
		opt.outlierSuccessRateMinHosts = defaultOutlierSuccessRateMinHosts
	}
}

type clusterDiscoveryService struct {
	opt       *cdsOpt
	xdsConfig *corev3.ConfigSource
	version   uint64
}

func (c *clusterDiscoveryService) increVersion() uint64 {
	return atomic.AddUint64(&c.version, 1)
}

func (c *clusterDiscoveryService) commonLbConfig(cfg CDSConfig) *clusterv3.Cluster_CommonLbConfig {
	return &clusterv3.Cluster_CommonLbConfig{
		// https://www.envoyproxy.io/docs/envoy/v1.15.0/intro/arch_overview/upstream/load_balancing/panic_threshold#arch-overview-load-balancing-panic-threshold
		HealthyPanicThreshold: &typev3.Percent{
			Value: float64(1.0),
		},
		//LocalityConfigSpecifier: &clusterv3.Cluster_CommonLbConfig_ZoneAwareLbConfig_{
		//	ZoneAwareLbConfig: &clusterv3.Cluster_CommonLbConfig_ZoneAwareLbConfig{
		//		MinClusterSize: &wrappers.UInt64Value{Value: 1},
		//		// https://github.com/envoyproxy/go-control-plane/blob/93f60a98b5b2f187be679be132acff5633a4d2e8/envoy/config/cluster/v3/cluster.pb.go#L2591-L2594
		//		FailTrafficOnPanic: false,
		//	},
		//},
		LocalityConfigSpecifier: &clusterv3.Cluster_CommonLbConfig_LocalityWeightedLbConfig_{
			LocalityWeightedLbConfig: new(clusterv3.Cluster_CommonLbConfig_LocalityWeightedLbConfig),
		},
	}
}

func (c *clusterDiscoveryService) subsetLbConfig(cfg CDSConfig) *clusterv3.Cluster_LbSubsetConfig {
	return &clusterv3.Cluster_LbSubsetConfig{
		FallbackPolicy:      clusterv3.Cluster_LbSubsetConfig_ANY_ENDPOINT,
		LocalityWeightAware: true,
		ScaleLocalityWeight: true,
	}
}

func (c *clusterDiscoveryService) clusterRefreshRate(cfg CDSConfig) *clusterv3.Cluster_RefreshRate {
	return &clusterv3.Cluster_RefreshRate{
		BaseInterval: ptypes.DurationProto(c.opt.clusterRefreshIntervalBase),
		MaxInterval:  ptypes.DurationProto(c.opt.clusterRefreshIntervalMax),
	}
}

func (c *clusterDiscoveryService) lbPolicy(cfg CDSConfig) clusterv3.Cluster_LbPolicy {
	switch cfg.LbPolicy {
	case "round-robin":
		return clusterv3.Cluster_ROUND_ROBIN
	case "least-reqest":
		return clusterv3.Cluster_LEAST_REQUEST
	case "random":
		return clusterv3.Cluster_RANDOM
	default:
		return clusterv3.Cluster_ROUND_ROBIN
	}
}

func (c *clusterDiscoveryService) clusters(configs []CDSConfig) []*clusterv3.Cluster {
	clusters := make([]*clusterv3.Cluster, len(configs))
	for idx, config := range configs {
		clusters[idx] = c.clusterConfig(config)
	}
	return clusters
}

func (c *clusterDiscoveryService) clusterConfig(cfg CDSConfig) *clusterv3.Cluster {
	// ref: rds.cluster
	clusterName := xdsName("example-xds-cluster", cfg.ClusterName)
	return &clusterv3.Cluster{
		Name:                      clusterName,
		ConnectTimeout:            ptypes.DurationProto(c.opt.clusterConnectionTimeout),
		UpstreamConnectionOptions: c.upstreamConnectionOptions(),
		ClusterDiscoveryType:      &clusterv3.Cluster_Type{Type: clusterv3.Cluster_EDS},
		EdsClusterConfig:          c.edsConfig(cfg.ClusterName),
		CommonLbConfig:            c.commonLbConfig(cfg),
		LbSubsetConfig:            c.subsetLbConfig(cfg),
		LbPolicy:                  c.lbPolicy(cfg),
		DnsLookupFamily:           clusterv3.Cluster_AUTO,
		DnsFailureRefreshRate:     c.clusterRefreshRate(cfg),
		RespectDnsTtl:             true,
		HealthChecks:              c.healthChecks(cfg.HealthCheck),
		// https://github.com/envoyproxy/go-control-plane/blob/93f60a98b5b2f187be679be132acff5633a4d2e8/envoy/config/cluster/v3/cluster.pb.go#L774-L777
		IgnoreHealthOnHostRemoval: true,
		OutlierDetection:          c.outlierDetection(cfg),
	}
}

func (c *clusterDiscoveryService) upstreamConnectionOptions() *clusterv3.UpstreamConnectionOptions {
	return &clusterv3.UpstreamConnectionOptions{
		TcpKeepalive: &corev3.TcpKeepalive{
			KeepaliveTime:     &wrappers.UInt32Value{Value: c.opt.clusterUpstreamKeepAliveTimeSeconds},
			KeepaliveInterval: &wrappers.UInt32Value{Value: c.opt.clusterUpstreamKeepAliveIntervalSeconds},
		},
	}
}

func (c *clusterDiscoveryService) healthCheckBase(cfg CDSHealthCheckConfig) *corev3.HealthCheck {
	return &corev3.HealthCheck{
		Timeout:            ptypes.DurationProto(cfg.TimeoutSecond()),
		Interval:           ptypes.DurationProto(cfg.IntervalSecond()),
		HealthyThreshold:   &wrappers.UInt32Value{Value: cfg.HealthyCount},
		UnhealthyThreshold: &wrappers.UInt32Value{Value: cfg.UnhealthyCount},
		InitialJitter:      ptypes.DurationProto(c.opt.healthCheckInitialJitter),
		NoTrafficInterval:  ptypes.DurationProto(c.opt.healthCheckInitialIntervalOnAddCluster),
	}
}

func (c *clusterDiscoveryService) healthChecks(cfg CDSHealthCheckConfig) []*corev3.HealthCheck {
	hc := c.healthCheckBase(cfg)
	hc.HealthChecker = c.healthCheckerHttp(cfg)
	return []*corev3.HealthCheck{hc}
}

func (c *clusterDiscoveryService) expectedHttpStatusOk() []*typev3.Int64Range {
	return []*typev3.Int64Range{
		&typev3.Int64Range{
			Start: 200,
			End:   299,
		},
	}
}

func (c *clusterDiscoveryService) statusesInt64Range(cfg CDSHealthCheckConfig) []*typev3.Int64Range {
	if len(cfg.Status) < 1 {
		// default status: 2xx only
		return c.expectedHttpStatusOk()
	}

	statusUint64 := make([]uint64, len(cfg.Status))
	for i, statusStr := range cfg.Status {
		statusUint, err := strconv.ParseUint(statusStr, 10, 32)
		if err != nil {
			log.Printf("warn: status string parse error fallback status200: %s", err.Error())
			statusUint = uint64(200)
		}
		statusUint64[i] = statusUint
	}

	if len(statusUint64) == 1 {
		// [200] only
		if statusUint64[0] == 200 {
			return c.expectedHttpStatusOk()
		}
		return []*typev3.Int64Range{
			&typev3.Int64Range{
				Start: int64(statusUint64[0]),
				End:   int64(statusUint64[0] + 1),
			},
		}
	}

	sort.Slice(statusUint64, func(i, j int) bool {
		return statusUint64[i] < statusUint64[j]
	})
	if statusUint64[0] == statusUint64[len(statusUint64)-1] {
		// avoid same value
		statusUint64[len(statusUint64)-1] += 1
	}

	statuses := make([]*typev3.Int64Range, len(statusUint64))
	for i, status := range statusUint64 {
		statuses[i] = &typev3.Int64Range{
			Start: int64(status),
			End:   int64(status + 1),
		}
	}
	return statuses
}

func (c *clusterDiscoveryService) healthCheckerHttp(cfg CDSHealthCheckConfig) *corev3.HealthCheck_HttpHealthCheck_ {
	return &corev3.HealthCheck_HttpHealthCheck_{
		HttpHealthCheck: &corev3.HealthCheck_HttpHealthCheck{
			Host:             cfg.Host,
			Path:             cfg.Path,
			ExpectedStatuses: c.statusesInt64Range(cfg),
		},
	}
}

func (c *clusterDiscoveryService) edsConfig(usage string) *clusterv3.Cluster_EdsClusterConfig {
	// ref: eds.clusterLoadAssignment
	edsServiceName := xdsName("example-xds-eds", usage)
	return &clusterv3.Cluster_EdsClusterConfig{
		ServiceName: edsServiceName,
		EdsConfig:   c.xdsConfig,
	}
}

func (c *clusterDiscoveryService) outlierDetection(cfg CDSConfig) *clusterv3.OutlierDetection {
	// https://github.com/envoyproxy/go-control-plane/blob/93f60a98b5b2f187be679be132acff5633a4d2e8/envoy/config/cluster/v3/outlier_detection.pb.go#L40-L43
	return &clusterv3.OutlierDetection{
		Consecutive_5Xx:           &wrappers.UInt32Value{Value: c.opt.outlierConsecutiveCount5xx},
		ConsecutiveGatewayFailure: &wrappers.UInt32Value{Value: c.opt.outlierConsecutiveGatewayFailure},
		Interval:                  ptypes.DurationProto(c.opt.outlierInterval),
		BaseEjectionTime:          ptypes.DurationProto(c.opt.outlierBaseEjectionTime),
		SuccessRateMinimumHosts:   &wrappers.UInt32Value{Value: c.opt.outlierSuccessRateMinHosts},
	}
}

func (c *clusterDiscoveryService) create(configs []CDSConfig) (string, []*clusterv3.Cluster, error) {
	version := strconv.FormatUint(c.increVersion(), 10)
	return version, c.clusters(configs), nil
}

func newClusterDiscoveryService(xdsConfig *corev3.ConfigSource, funcs ...cdsOptFunc) *clusterDiscoveryService {
	opt := new(cdsOpt)
	for _, fn := range funcs {
		fn(opt)
	}
	initCdsOpt(opt)

	return &clusterDiscoveryService{
		opt:       opt,
		xdsConfig: xdsConfig,
		version:   uint64(0),
	}
}
