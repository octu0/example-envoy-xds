package xds

import (
	"time"
)

type CDSConfig struct {
	ClusterName string               `yaml:"name"         validate:"required"`
	LbPolicy    string               `yaml:"lb-policy"    validate:"required"`
	HealthCheck CDSHealthCheckConfig `yaml:"health-check" validate:"required"`
}

type CDSHealthCheckConfig struct {
	Host           string   `yaml:"host"         validate:""`
	Path           string   `yaml:"path"         validate:"required"`
	Status         []string `yaml:"status"       validate:"required,unique"`
	Timeout        uint32   `yaml:"timeout"      validate:"gte=1,lte=900"`
	Interval       uint32   `yaml:"interval"     validate:"gte=1,lte=180"`
	HealthyCount   uint32   `yaml:"healthy"      validate:"gte=1,lte=10"`
	UnhealthyCount uint32   `yaml:"unhealthy"    validate:"gte=1,lte=10"`
}

func (c CDSHealthCheckConfig) TimeoutSecond() time.Duration {
	return time.Duration(c.Timeout) * time.Second
}

func (c CDSHealthCheckConfig) IntervalSecond() time.Duration {
	return time.Duration(c.Interval) * time.Second
}
