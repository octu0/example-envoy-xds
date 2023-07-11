package xds

import (
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
)

type EDSConfig struct {
	ClusterName     string              `yaml:"name"             validate:"required"`
	BalancingPolicy string              `yaml:"balancing-policy" validate:"required"`
	Instances       []EDSInstanceConfig `yaml:"instances"        validate:"required"`
}

type EDSInstanceConfig struct {
	InstanceName string `yaml:"instance-name"  validate:"required"`
	IP           string `yaml:"ip"             validate:"required,ip"`
	Port         uint32 `yaml:"port"           validate:"required,gte=1,lte=65535"`
	Region       string `yaml:"region"         validate:"required"`
	Zone         string `yaml:"zone"           validate:"zone"`
	Protocol     string `yaml:"protocol"       validate:"required"`
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
