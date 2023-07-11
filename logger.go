package xds

import (
	"log"
	"os"

	envoylog "github.com/envoyproxy/go-control-plane/pkg/log"
)

// compile check
var (
	_ envoylog.Logger = (*loggerSnapshotCache)(nil)
)

type loggerSnapshotCache struct{}

func (l *loggerSnapshotCache) Debugf(format string, args ...interface{}) {
	log.Printf("debug: "+format, args...)
}

func (l *loggerSnapshotCache) Infof(format string, args ...interface{}) {
	log.Printf("info: "+format, args...)
}

func (l *loggerSnapshotCache) Warnf(format string, args ...interface{}) {
	log.Printf("warn: "+format, args...)
}

func (l *loggerSnapshotCache) Errorf(format string, args ...interface{}) {
	log.Printf("error: "+format, args...)
}

func newLoggerSnapshotCache() *loggerSnapshotCache {
	return new(loggerSnapshotCache)
}

func newLoggerAccessLog() *log.Logger {
	// todo
	return log.New(os.Stdout, "accesslog: ", log.Ldate|log.Lmicroseconds)
}
