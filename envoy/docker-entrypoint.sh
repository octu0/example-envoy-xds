#!/usr/bin/dumb-init /bin/sh
set -e

cp /etc/envoy/envoy.yaml /envoy/envoy.yaml

cluster=${ENVOY_XDS_CLUSTER:-"example0"}
node=${ENVOY_XDS_NODE_ID:-"node0"}
host=${ENVOY_XDS_HOST:-"127.0.0.1"}
port=${ENVOY_XDS_PORT:-"5000"}
region=${ENVOY_XDS_LOCALITY_REGION:-"ane"}
zone=${ENVOY_XDS_LOCALITY_ZONE:-"1a"}
alshost=${ENVOY_ALS_HOST:-"127.0.0.1"}
alsport=${ENVOY_ALS_PORT:-"5001"}
adminhost=${ENVOY_ADMIN_LISTEN_HOST:-"127.0.0.1"}
adminport=${ENVOY_ADMIN_LISTEN_PORT:-"9800"}

sed -i -e "s/@ENVOY_XDS_CLUSTER@/$cluster/" /envoy/envoy.yaml
sed -i -e "s/@ENVOY_XDS_NODE_ID@/$node/" /envoy/envoy.yaml
sed -i -e "s/@ENVOY_XDS_HOST@/$host/" /envoy/envoy.yaml
sed -i -e "s/@ENVOY_XDS_PORT@/$port/" /envoy/envoy.yaml
sed -i -e "s/@ENVOY_XDS_LOCALITY_REGION@/$region/" /envoy/envoy.yaml
sed -i -e "s/@ENVOY_XDS_LOCALITY_ZONE@/$zone/" /envoy/envoy.yaml
sed -i -e "s/@ENVOY_ALS_HOST@/$alshost/" /envoy/envoy.yaml
sed -i -e "s/@ENVOY_ALS_PORT@/$alsport/" /envoy/envoy.yaml
sed -i -e "s/@ENVOY_ADMIN_LISTEN_HOST@/$adminhost/" /envoy/envoy.yaml
sed -i -e "s/@ENVOY_ADMIN_LISTEN_PORT@/$adminport/" /envoy/envoy.yaml

exec envoy -c /envoy/envoy.yaml "$@"
