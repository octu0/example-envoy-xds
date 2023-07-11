package xds

import (
	"time"
)

type RDSConfig struct {
	VHostName string             `yaml:"vhost"   validate:"required"`
	Domain    []string           `yaml:"domain"  validate:"required,unique"`
	Cluster   []RDSClusterConfig `yaml:"cluster" validate:"required"`
	Action    RDSActionConfig    `yaml:"action"  validate:"required"`
}

type RDSClusterConfig struct {
	Prefix  string                   `yaml:"prefix"  validate:"required"`
	Target  []RDSClusterWeightConfig `yaml:"target"  validate:"required"`
	Headers []RDSClusterHeaderConfig `yaml:"headers" validate:""`
}

type RDSClusterWeightConfig struct {
	ClusterName string `yaml:"name"   validate:"required"`
	Weight      uint32 `yaml:"weight" validate:"gte=0,lte=100"`
}

type RDSClusterHeaderConfig struct {
	HeaderName  string           `yaml:"name"          validate:""`
	StringMatch RDSStringMatcher `yaml:"string_match"  validate:""`
}

type RDSStringMatcher struct {
	Exact string `yaml:"exact" validate:""`
}

type RDSActionConfig struct {
	Timeout     uint32 `yaml:"timeout"      validate:"required"`
	IdleTimeout uint32 `yaml:"idle-timeout" validate:"required"`
	RetryPolicy string `yaml:"retry-policy" validate:"required"`
}

func (c RDSActionConfig) TimeoutSecond() time.Duration {
	return time.Duration(c.Timeout) * time.Second
}

func (c RDSActionConfig) IdleTimeoutSecond() time.Duration {
	return time.Duration(c.IdleTimeout) * time.Second
}
