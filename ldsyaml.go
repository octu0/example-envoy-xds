package xds

import (
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
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
