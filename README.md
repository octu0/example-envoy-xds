# `example-envoy-xds`

[![Apache License](https://img.shields.io/github/license/octu0/example-envoy-xds)](https://github.com/octu0/example-envoy-xds/blob/master/LICENSE)
[![GoDoc](https://godoc.org/github.com/octu0/example-envoy-xds?status.svg)](https://godoc.org/github.com/octu0/example-envoy-xds)
[![Go Report Card](https://goreportcard.com/badge/github.com/octu0/example-envoy-xds)](https://goreportcard.com/report/github.com/octu0/example-envoy-xds)
[![Releases](https://img.shields.io/github/v/release/octu0/example-envoy-xds)](https://github.com/octu0/example-envoy-xds/releases)

`example-envoy-xds` is an example of implementation of [envoy](https://www.envoyproxy.io/) and [control-plane](https://github.com/envoyproxy/go-control-plane/) using [v3 xDS](https://www.envoyproxy.io/docs/envoy/v1.15.3/api-docs/xds_protocol) API.

Features:
- xDS (EDS/CDS/LDS/RDS/ALS)
- Dynamic update of yaml files (using [fsnotify](github.com/fsnotify/fsnotify))
- Access log storage using ALS
- Configuration examples of various settings
- Configuration of Weighted Round Robin LoadBalancer

## Bootstraping

As bootstrap, in [envoy/envoy.yaml](https://github.com/octu0/example-envoy-xds/blob/master/envoy/envoy.yaml), specify `example-envoy-xds` in `xds_cluster` and `als_cluster`  
This will allow xDS communication with grpc.

For general use, `envoy.yaml` is used as a template file and replaced by `sed` in [docker-entrypoint.sh](https://github.com/octu0/example-envoy-xds/blob/master/envoy/docker-entrypoint.sh).

```yaml
node:
  cluster: @ENVOY_XDS_CLUSTER@
  id: @ENVOY_XDS_NODE_ID@
  locality:
    region: @ENVOY_XDS_LOCALITY_REGION@
    zone: @ENVOY_XDS_LOCALITY_ZONE@

admin:
  access_log_path: /dev/null
  address:
    socket_address: { protocol: TCP, address: @ENVOY_ADMIN_LISTEN_HOST@, port_value: @ENVOY_ADMIN_LISTEN_PORT@ }

dynamic_resources:
  lds_config:
    resource_api_version: V3
    api_config_source:
      api_type: GRPC
      transport_api_version: V3
      grpc_services:
      - envoy_grpc: { cluster_name: xds_cluster }
      set_node_on_first_message_only: true
  cds_config:
    resource_api_version: V3
    api_config_source:
      api_type: GRPC
      transport_api_version: V3
      grpc_services:
      - envoy_grpc: { cluster_name: xds_cluster }
      set_node_on_first_message_only: true

static_resources:
  clusters:
  - name: xds_cluster
    connect_timeout: 1s
    type: STATIC
    lb_policy: ROUND_ROBIN
    http2_protocol_options: {}
    load_assignment:
      cluster_name: xds_cluster
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address: { protocol: TCP, address: @ENVOY_XDS_HOST@, port_value: @ENVOY_XDS_PORT@ }
  - name: als_cluster
    connect_timeout: 1s
    type: STATIC
    lb_policy: ROUND_ROBIN
    http2_protocol_options: {}
    upstream_connection_options:
      tcp_keepalive: {}
    load_assignment:
      cluster_name: als_cluster
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address: { protocol: TCP, address: @ENVOY_ALS_HOST@, port_value: @ENVOY_ALS_PORT@ }

layered_runtime:
  layers:
    - name: runtime0
      rtds_layer:
        rtds_config:
          resource_api_version: V3
          api_config_source:
            transport_api_version: V3
            api_type: GRPC
            grpc_services:
              envoy_grpc:
                cluster_name: xds_cluster
        name: runtime0
```

When you start envoy with docker, you can specify the IP and port of `example-envoy-xds` with environment variables.

```shell
$ docker run --net=host                          \
  -e ENVOY_XDS_CLUSTER=example0                  \
  -e ENVOY_XDS_NODE_ID=envoy-node1               \
  -e ENVOY_XDS_HOST=10.10.0.101                  \
  -e ENVOY_XDS_PORT=5000                         \
  -e ENVOY_XDS_LOCALITY_REGION=asia-northeast1   \
  -e ENVOY_XDS_LOCALITY_ZONE=asia-northeast1-a   \
  -e ENVOY_ALS_HOST=10.10.0.101                  \
  -e ENVOY_ALS_PORT=5001                         \
  docker.pkg.github.com/octu0/example-envoy-xds/envoy:1.15.3
```

Configure xDS with grpc, `example-envoy-xds` will be started so that envoy can communicate with it.  
At this time, the node.id of envoy (specified by `ENVOY_XDS_NODE_ID`) must be the same value to start, otherwise the snapshot will not be changed.

```shell
$ docker run --net=host           \
  -e XDS_NODE_ID=envoy-node1      \
  -e XDS_LISTEN_ADDR=0.0.0.0:5000 \
  -e ALS_LISTEN_ADDR=0.0.0.0:5001 \
  -e CDS_YAML=/app/vol/cds.yaml   \
  -e EDS_YAML=/app/vol/eds.yaml   \
  -e RDS_YAML=/app/vol/rds.yaml   \
  -e LDS_YAML=/app/vol/lds.yaml   \
  -v $(pwd):/app/vol              \
  docker.pkg.github.com/octu0/example-envoy-xds/example-envoy-xds:1.0.0 server
```

## Execution example

Using docker-compose to check the behavior. 

```shell
$ docker-compose up -d
```

and curl it.

```shell
$ curl -H 'Host:example.com' localhost:8080
legacy node-003

$ curl -H 'Host:example.com' localhost:8080
new node-102
```

## License

Apache 2.0, see LICENSE file for details.
