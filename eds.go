package xds

import (
	"log"
	"strconv"
	"sync/atomic"

	"github.com/golang/protobuf/ptypes/wrappers"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
)

type EDSConfig struct {
	ClusterName     string              `yaml:"name"             validate:"required"`
	BalancingPolicy string              `yaml:"balancing-policy" validate:"required"`
	Instances       []EDSInstanceConfig `yaml:"instances"        validate:"required"`
}

type EDSInstanceConfig struct {
	InstanceName string `yaml:"instance-name"    validate:"required"`
	IP           string `yaml:"ip"               validate:"required,ip"`
	Port         uint32 `yaml:"port"             validate:"required,gte=1,lte=65535"`
	Region       string `yaml:"region"           validate:"required"`
	Zone         string `yaml:"zone"             validate:"zone"`
	Protocol     string `yaml:"protocol"         validate:"required"`
}

func (c EDSInstanceConfig) Address() *corev3.Address {
	switch c.Protocol {
	case "tcp":
		return c.addr(corev3.SocketAddress_TCP)
	case "udp":
		return c.addr(corev3.SocketAddress_UDP)
	default:
		return c.addr(corev3.SocketAddress_TCP)
	}
}

func (c EDSInstanceConfig) addr(protocol corev3.SocketAddress_Protocol) *corev3.Address {
	return &corev3.Address{
		Address: &corev3.Address_SocketAddress{
			SocketAddress: &corev3.SocketAddress{
				//ResolverName: "STRICT_DNS", // if hostname is included in the IP specification, use DNS.
				Protocol: protocol,
				Address:  c.IP,
				PortSpecifier: &corev3.SocketAddress_PortValue{
					PortValue: c.Port,
				},
			},
		},
	}
}

const (
	defaultLoadBalancingWeight uint32 = 1
)

type edsOptFunc func(*edsOpt)

type edsOpt struct {
	loadBalancingWeight uint32
}

func EdsLoadBalancingWeight(weight uint32) edsOptFunc {
	return func(opt *edsOpt) {
		opt.loadBalancingWeight = weight
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
	endpoints := make([]*endpointv3.LbEndpoint, len(instances))
	for idx, ins := range instances {
		endpoints[idx] = &endpointv3.LbEndpoint{
			HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
				Endpoint: e.instanceEndpoint(ins),
			},
			HealthStatus: corev3.HealthStatus_UNHEALTHY, // initial status = unhealthy
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

func instancesByRegion(instances []EDSInstanceConfig) map[string][]EDSInstanceConfig {
	m := make(map[string][]EDSInstanceConfig)
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
	m := make(map[string][]EDSInstanceConfig)
	for _, ins := range instances {
		if exists, ok := m[ins.Zone]; ok != true {
			m[ins.Zone] = []EDSInstanceConfig{ins}
		} else {
			m[ins.Zone] = append(exists, ins)
		}
	}
	return m
}
