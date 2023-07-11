package xds

import (
	"log"
	"strconv"
	"sync/atomic"

	"github.com/golang/protobuf/ptypes/wrappers"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
)

const (
	defaultLoadBalancingWeight uint32 = 1
)

type edsOptFunc func(*edsOpt)

type edsOpt struct {
	loadBalancingWeight    uint32
	healthStatus           corev3.HealthStatus
	setInitialHealthStatus bool
}

func EdsLoadBalancingWeight(weight uint32) edsOptFunc {
	return func(opt *edsOpt) {
		opt.loadBalancingWeight = weight
	}
}

func EdsLbEndpointHealthStatus(status corev3.HealthStatus) edsOptFunc {
	return func(opt *edsOpt) {
		opt.healthStatus = status
		opt.setInitialHealthStatus = true
	}
}

func initEdsOpt(opt *edsOpt) {
	if opt.loadBalancingWeight < 1 {
		opt.loadBalancingWeight = defaultLoadBalancingWeight
	}
}

type endpointDiscoveryService struct {
	opt       *edsOpt
	xdsConfig *corev3.ConfigSource
	version   uint64
}

func (e *endpointDiscoveryService) increVersion() uint64 {
	return atomic.AddUint64(&e.version, 1)
}

func (e *endpointDiscoveryService) instanceEndpoint(instance EDSInstanceConfig) *endpointv3.Endpoint {
	log.Printf("info: endpoint protocol=%s instance=%s ip=%s port=%d", instance.Protocol, instance.InstanceName, instance.IP, instance.Port)
	return &endpointv3.Endpoint{
		Address:  instance.Address(),
		Hostname: instance.InstanceName,
		HealthCheckConfig: &endpointv3.Endpoint_HealthCheckConfig{
			PortValue: instance.Port,
		},
	}
}

func (e *endpointDiscoveryService) lbEndpoints(instances []EDSInstanceConfig) []*endpointv3.LbEndpoint {
	if e.opt.setInitialHealthStatus {
		return e.lbEndpointsWithInitialStatus(instances, e.opt.healthStatus)
	}
	return e.lbEndpointsDefault(instances)
}

func (e *endpointDiscoveryService) lbEndpointsWithInitialStatus(instances []EDSInstanceConfig, status corev3.HealthStatus) []*endpointv3.LbEndpoint {
	endpoints := make([]*endpointv3.LbEndpoint, len(instances))
	for idx, ins := range instances {
		endpoints[idx] = &endpointv3.LbEndpoint{
			HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
				Endpoint: e.instanceEndpoint(ins),
			},
			HealthStatus: status,
		}
	}
	return endpoints
}

func (e *endpointDiscoveryService) lbEndpointsDefault(instances []EDSInstanceConfig) []*endpointv3.LbEndpoint {
	endpoints := make([]*endpointv3.LbEndpoint, len(instances))
	for idx, ins := range instances {
		endpoints[idx] = &endpointv3.LbEndpoint{
			HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
				Endpoint: e.instanceEndpoint(ins),
			},
		}
	}
	return endpoints
}

func (e *endpointDiscoveryService) lbLocalityEndpoint(region string, zone string, instances []EDSInstanceConfig) *endpointv3.LocalityLbEndpoints {
	for _, mri := range instances {
		log.Printf("info: locality endpoint: region=%s zone=%s instance=%s", region, zone, mri.InstanceName)
	}
	return &endpointv3.LocalityLbEndpoints{
		// https://github.com/envoyproxy/go-control-plane/blob/0876eda0031110bd8b32e221899ad015a5365e1b/envoy/config/core/v3/base.pb.go#L213
		Locality: &corev3.Locality{
			Region: region,
			Zone:   zone,
		},
		LbEndpoints: e.lbEndpoints(instances),
		// https://www.envoyproxy.io/docs/envoy/v1.15.0/intro/arch_overview/upstream/load_balancing/locality_weight#arch-overview-load-balancing-locality-weighted-lb
		// https://www.envoyproxy.io/docs/envoy/v1.15.0/api-v3/config/endpoint/v3/endpoint_components.proto#envoy-v3-api-msg-config-endpoint-v3-localitylbendpoints
		LoadBalancingWeight: &wrappers.UInt32Value{Value: e.opt.loadBalancingWeight},
	}
}

func (e *endpointDiscoveryService) lbLocalityEndpointsByRegion(region string, instances []EDSInstanceConfig) []*endpointv3.LocalityLbEndpoints {
	lbEndpoints := make([]*endpointv3.LocalityLbEndpoints, 0, len(instances))
	for zone, ins := range instancesByZone(instances) {
		lbEndpoints = append(lbEndpoints, e.lbLocalityEndpoint(region, zone, ins))
	}
	return lbEndpoints
}

func (e *endpointDiscoveryService) lbLocalityEnpoints(instances []EDSInstanceConfig) []*endpointv3.LocalityLbEndpoints {
	lbEndpoints := make([]*endpointv3.LocalityLbEndpoints, 0, len(instances))
	for region, ins := range instancesByRegion(instances) {
		for _, lbEndpoint := range e.lbLocalityEndpointsByRegion(region, ins) {
			lbEndpoints = append(lbEndpoints, lbEndpoint)
		}
	}
	return lbEndpoints
}

func (e *endpointDiscoveryService) lbNormalEndpoints(instances []EDSInstanceConfig) []*endpointv3.LocalityLbEndpoints {
	return []*endpointv3.LocalityLbEndpoints{
		&endpointv3.LocalityLbEndpoints{
			LbEndpoints: e.lbEndpoints(instances),
		},
	}
}

func (e *endpointDiscoveryService) lbBalancingEndpoints(balancingPolicy string, instances []EDSInstanceConfig) []*endpointv3.LocalityLbEndpoints {
	switch balancingPolicy {
	case "normal":
		return e.lbNormalEndpoints(instances)
	case "locality":
		return e.lbLocalityEnpoints(instances)
	default:
		return e.lbLocalityEnpoints(instances)
	}
}

func (e *endpointDiscoveryService) clusterLoadAssignment(usage string, balancingPolicy string, instances []EDSInstanceConfig) *endpointv3.ClusterLoadAssignment {
	// ref: rds.cluster
	clusterName := xdsName("example-xds-eds", usage)
	return &endpointv3.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints:   e.lbBalancingEndpoints(balancingPolicy, instances),
	}
}

func (e *endpointDiscoveryService) edsEndpoints(configs []EDSConfig) []*endpointv3.ClusterLoadAssignment {
	endpoints := make([]*endpointv3.ClusterLoadAssignment, len(configs))
	for idx, config := range configs {
		endpoints[idx] = e.clusterLoadAssignment(config.ClusterName, config.BalancingPolicy, config.Instances)
	}
	return endpoints
}

func (e *endpointDiscoveryService) create(configs []EDSConfig) (string, []*endpointv3.ClusterLoadAssignment, error) {
	version := strconv.FormatUint(e.increVersion(), 10)
	return version, e.edsEndpoints(configs), nil
}

func newEndpointDiscoveryService(xdsConfig *corev3.ConfigSource, funcs ...edsOptFunc) *endpointDiscoveryService {
	opt := new(edsOpt)
	for _, fn := range funcs {
		fn(opt)
	}
	initEdsOpt(opt)

	return &endpointDiscoveryService{
		opt:       opt,
		xdsConfig: xdsConfig,
		version:   uint64(0),
	}
}

func instancesByRegion(instances []EDSInstanceConfig) map[string][]EDSInstanceConfig {
	m := make(map[string][]EDSInstanceConfig, len(instances))
	for _, ins := range instances {
		if exists, ok := m[ins.Region]; ok != true {
			m[ins.Region] = []EDSInstanceConfig{ins}
		} else {
			m[ins.Region] = append(exists, ins)
		}
	}
	return m
}

func instancesByZone(instances []EDSInstanceConfig) map[string][]EDSInstanceConfig {
	m := make(map[string][]EDSInstanceConfig, len(instances))
	for _, ins := range instances {
		if exists, ok := m[ins.Zone]; ok != true {
			m[ins.Zone] = []EDSInstanceConfig{ins}
		} else {
			m[ins.Zone] = append(exists, ins)
		}
	}
	return m
}
