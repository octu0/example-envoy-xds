package xds

import (
	"context"
	"io/ioutil"
	"log"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/go-playground/validator.v9"
	"gopkg.in/yaml.v2"

	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
)

type watchOptFunc func(*watchOpt)

type watchOpt struct {
	cdsYaml string
	edsYaml string
	rdsYaml string
	ldsYaml string
}

func WatchCdsConfigFile(path string) watchOptFunc {
	return func(opt *watchOpt) {
		opt.cdsYaml = path
	}
}

func WatchEdsConfigFile(path string) watchOptFunc {
	return func(opt *watchOpt) {
		opt.edsYaml = path
	}
}

func WatchRdsConfigFile(path string) watchOptFunc {
	return func(opt *watchOpt) {
		opt.rdsYaml = path
	}
}

func WatchLdsConfigFile(path string) watchOptFunc {
	return func(opt *watchOpt) {
		opt.ldsYaml = path
	}
}

type WatchFile struct {
	ctx context.Context
	nodeId   string
	opt      *watchOpt
	cache    cachev3.SnapshotCache
	cds      *clusterDiscoveryService
	eds      *endpointDiscoveryService
	rds      *routeDiscoveryService
	lds      *listenerDiscoveryService
	resource *resource
}

func NewWatchFile(ctx context.Context, nodeId string, funcs ...watchOptFunc) *WatchFile {
	opt := new(watchOpt)
	for _, fn := range funcs {
		fn(opt)
	}

	xdsConfig := xdsConfigSource()
	return &WatchFile{
		ctx:      ctx,
		nodeId:   nodeId,
		opt:      opt,
		cache:    cachev3.NewSnapshotCache(false, cachev3.IDHash{}, newLoggerSnapshotCache()),
		cds:      newClusterDiscoveryService(xdsConfig),
		eds:      newEndpointDiscoveryService(xdsConfig),
		rds:      newRouteDiscoveryService(xdsConfig),
		lds:      newListenerDiscoveryService(xdsConfig),
		resource: newResource(),
	}
}

func (w *WatchFile) Cache() cachev3.Cache {
	return w.cache
}

func (w *WatchFile) Watch(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := watcher.Add(w.opt.cdsYaml); err != nil {
		return err
	}
	if err := watcher.Add(w.opt.edsYaml); err != nil {
		return err
	}
	if err := watcher.Add(w.opt.ldsYaml); err != nil {
		return err
	}
	if err := watcher.Add(w.opt.rdsYaml); err != nil {
		return err
	}

	go w.watchFileLoop(ctx, watcher)
	return nil
}

func (w *WatchFile) watchFileLoop(ctx context.Context, watcher *fsnotify.Watcher) {
	defer log.Printf("info: stop file watching")

	for {
		select {
		case <-ctx.Done():
			return

		case err, ok := <-watcher.Errors:
			if ok != true {
				return
			}
			log.Printf("error: %s", err)

		case evt, ok := <-watcher.Events:
			if ok != true {
				return
			}

			if evt.Op == fsnotify.Rename {
				time.Sleep(100 * time.Millisecond) // wait file writes (todo retry)
			}

			if evt.Op != fsnotify.Chmod {
				log.Printf("info: file changed: %s(%s)", evt.Name, evt.Op)
				if equalPath(evt.Name, w.opt.cdsYaml) {
					if err := w.changeCdsYaml(); err != nil {
						log.Printf("warn: %s", err)
					}
				}
				if equalPath(evt.Name, w.opt.edsYaml) {
					if err := w.changeEdsYaml(); err != nil {
						log.Printf("warn: %s", err)
					}
				}
				if equalPath(evt.Name, w.opt.rdsYaml) {
					if err := w.changeRdsYaml(); err != nil {
						log.Printf("warn: %s", err)
					}
				}
				if equalPath(evt.Name, w.opt.ldsYaml) {
					if err := w.changeLdsYaml(); err != nil {
						log.Printf("warn: %s", err)
					}
				}
			}

			// recursive watch
			if err := watcher.Add(evt.Name); err != nil {
				log.Printf("error: failed add watch(%s) to fsnotify: %s", evt.Name, err.Error())
				return
			}
		}
	}
}

func (w *WatchFile) changeCdsYaml() error {
	config, err := w.loadCds()
	if err != nil {
		log.Printf("info: load CDS failed: %s", err)
		return err
	}

	if err := w.updateCds(config); err != nil {
		log.Printf("info: update CDS failed: %s", err)
		return err
	}
	log.Printf("info: update CDS succeed")

	if err := w.updateSnapshot(); err != nil {
		return err
	}
	return nil
}

func (w *WatchFile) changeEdsYaml() error {
	config, err := w.loadEds()
	if err != nil {
		log.Printf("info: load EDS failed: %s", err)
		return err
	}

	if err := w.updateEds(config); err != nil {
		log.Printf("info: update EDS failed: %s", err)
		return err
	}
	log.Printf("info: update EDS succeed")

	if err := w.updateSnapshot(); err != nil {
		return err
	}
	return nil
}

func (w *WatchFile) changeRdsYaml() error {
	config, err := w.loadRds()
	if err != nil {
		log.Printf("info: load RDS failed: %s", err)
		return err
	}

	if err := w.updateRds(config); err != nil {
		log.Printf("info: update RDS failed: %s", err)
		return err
	}
	log.Printf("info: update RDS succeed")

	if err := w.updateSnapshot(); err != nil {
		return err
	}
	return nil
}

func (w *WatchFile) changeLdsYaml() error {
	config, err := w.loadLds()
	if err != nil {
		log.Printf("info: load LDS failed: %s", err)
		return err
	}

	if err := w.updateLds(config); err != nil {
		log.Printf("info: update LDS failed: %s", err)
		return err
	}
	log.Printf("info: update LDS succeed")

	if err := w.updateSnapshot(); err != nil {
		return err
	}
	return nil
}

func (w *WatchFile) loadYaml(file string, bind interface{}) error {
	log.Printf("debug: load file: %s", file)

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, bind); err != nil {
		return err
	}
	return nil
}

func (w *WatchFile) loadCds() ([]CDSConfig, error) {
	configs := make([]CDSConfig, 0)
	if err := w.loadYaml(w.opt.cdsYaml, &configs); err != nil {
		return []CDSConfig{}, err
	}

	v := validator.New()
	for _, config := range configs {
		if err := v.Struct(config); err != nil {
			return []CDSConfig{}, err
		}
	}
	return configs, nil
}

func (w *WatchFile) loadEds() ([]EDSConfig, error) {
	configs := make([]EDSConfig, 0)
	if err := w.loadYaml(w.opt.edsYaml, &configs); err != nil {
		return []EDSConfig{}, err
	}

	v := validator.New()
	for _, config := range configs {
		if err := v.Struct(config); err != nil {
			return []EDSConfig{}, err
		}
	}
	return configs, nil
}

func (w *WatchFile) loadRds() ([]RDSConfig, error) {
	configs := make([]RDSConfig, 0)
	if err := w.loadYaml(w.opt.rdsYaml, &configs); err != nil {
		return []RDSConfig{}, err
	}

	v := validator.New()
	for _, config := range configs {
		if err := v.Struct(config); err != nil {
			return []RDSConfig{}, err
		}
	}
	return configs, nil
}

func (w *WatchFile) loadLds() (LDSConfig, error) {
	config := LDSConfig{}
	if err := w.loadYaml(w.opt.ldsYaml, &config); err != nil {
		return LDSConfig{}, err
	}

	v := validator.New()
	if err := v.Struct(config); err != nil {
		return LDSConfig{}, err
	}
	return config, nil
}

func (w *WatchFile) updateCds(config []CDSConfig) error {
	version, clusters, err := w.cds.create(config)
	if err != nil {
		return err
	}
	w.resource.updateCluster(version, clusters)
	return nil
}

func (w *WatchFile) updateEds(config []EDSConfig) error {
	version, endpoints, err := w.eds.create(config)
	if err != nil {
		return err
	}
	w.resource.updateEndpoint(version, endpoints)
	return nil
}

func (w *WatchFile) updateRds(config []RDSConfig) error {
	version, route, err := w.rds.create(config)
	if err != nil {
		return err
	}
	w.resource.updateRoute(version, route)
	return nil
}

func (w *WatchFile) updateLds(config LDSConfig) error {
	version, listener, err := w.lds.create(config)
	if err != nil {
		return err
	}
	w.resource.updateListener(version, listener)
	return nil
}

func (w *WatchFile) updateSnapshot() error {
	version, snapshot, err := w.resource.Snapshot()
	if err != nil {
		log.Printf("error: snapshot consistent error: %s", err.Error())
		return err
	}

	log.Printf("info: xds %s snapshot version: %s", w.nodeId, version)
	w.cache.SetSnapshot(w.ctx, w.nodeId, snapshot)
	return nil
}

// all or nothing reload
func (w *WatchFile) ReloadAll() error {
	cdsConfig, err := w.loadCds()
	if err != nil {
		return err
	}
	edsConfig, err := w.loadEds()
	if err != nil {
		return err
	}
	rdsConfig, err := w.loadRds()
	if err != nil {
		return err
	}
	ldsConfig, err := w.loadLds()
	if err != nil {
		return err
	}

	if err := w.updateCds(cdsConfig); err != nil {
		return err
	}
	if err := w.updateEds(edsConfig); err != nil {
		return err
	}
	if err := w.updateRds(rdsConfig); err != nil {
		return err
	}
	if err := w.updateLds(ldsConfig); err != nil {
		return err
	}

	if err := w.updateSnapshot(); err != nil {
		return err
	}
	return nil
}

func equalPath(src, target string) bool {
	absSrc, err := filepath.Abs(src)
	if err != nil {
		log.Printf("error: %s error: %s", src, err)
		absSrc = src
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		log.Printf("error: %s error: %s", target, err)
		absTarget = target
	}
	return absSrc == absTarget
}
