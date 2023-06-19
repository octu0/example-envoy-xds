package xds

import (
	"strings"
	"sync"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	typesv3 "github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
)

func versionString(endpoint, cluster, route, listener string) string {
	return strings.Join([]string{endpoint, cluster, route, listener}, ".")
}

type resource struct {
	mutex            *sync.RWMutex
	endpoints        []*endpointv3.ClusterLoadAssignment
	endpointsVersion string
	clusters         []*clusterv3.Cluster
	clustersVersion  string
	route            *routev3.RouteConfiguration
	routeVersion     string
	listener         *listenerv3.Listener
	listenerVersion  string
}

func newResource() *resource {
	return &resource{
		mutex:            new(sync.RWMutex),
		endpoints:        nil,
		endpointsVersion: "0",
		clusters:         nil,
		clustersVersion:  "0",
		route:            nil,
		routeVersion:     "0",
		listener:         nil,
		listenerVersion:  "0",
	}
}

func (r *resource) updateListener(version string, listener *listenerv3.Listener) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.listenerVersion = version
	r.listener = listener
}

func (r *resource) currentListener() (string, *listenerv3.Listener) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.listenerVersion, r.listener
}

func (r *resource) updateRoute(version string, route *routev3.RouteConfiguration) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.routeVersion = version
	r.route = route
}

func (r *resource) currentRoute() (string, *routev3.RouteConfiguration) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.routeVersion, r.route
}

func (r *resource) updateCluster(version string, clusters []*clusterv3.Cluster) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.clustersVersion = version
	r.clusters = clusters
}

func (r *resource) currentCluster() (string, []*clusterv3.Cluster) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.clustersVersion, r.clusters
}

func (r *resource) updateEndpoint(version string, endpoints []*endpointv3.ClusterLoadAssignment) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.endpointsVersion = version
	r.endpoints = endpoints
}

func (r *resource) currentEndpoint() (string, []*endpointv3.ClusterLoadAssignment) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.endpointsVersion, r.endpoints
}

func (r *resource) version() string {
	return versionString(
		r.endpointsVersion,
		r.clustersVersion,
		r.routeVersion,
		r.listenerVersion,
	)
}

func (r *resource) Snapshot() (string, *cachev3.Snapshot, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	endpoints := make([]typesv3.Resource, len(r.endpoints))
	for i, e := range r.endpoints {
		endpoints[i] = e
	}

	clusters := make([]typesv3.Resource, len(r.clusters))
	for i, c := range r.clusters {
		clusters[i] = c
	}

	version := r.version()

	snapshot, err := cachev3.NewSnapshot(
		version,
		map[resourcev3.Type][]typesv3.Resource{
			resourcev3.EndpointType: endpoints,
			resourcev3.ClusterType:  clusters,
			resourcev3.RouteType:    []typesv3.Resource{r.route},
			resourcev3.ListenerType: []typesv3.Resource{r.listener},
		},
	)
	if err != nil {
		return "", &cachev3.Snapshot{}, err
	}

	// validate variables
	if err := snapshot.Consistent(); err != nil {
		return "", &cachev3.Snapshot{}, err
	}
	return version, snapshot, nil
}
