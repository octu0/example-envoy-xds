#!/usr/bin/dumb-init /bin/sh
set -e

exec /app/example-xds-server "$@"
