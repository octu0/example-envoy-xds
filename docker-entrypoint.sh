#!/usr/bin/dumb-init /bin/sh
set -e

su-exec xds:xds /app/example-xds-server "$@"
