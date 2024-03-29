FROM golang:1.20-alpine as builder

WORKDIR /build
ADD . /go/src/github.com/octu0/example-xds-server

RUN set -eux && \
    apk add --no-cache --virtual .build-deps git make openssh-client && \
    cd /go/src/github.com/octu0/example-xds-server && \
    GOOS=linux GOARCH=amd64 go build -a \
      -tags netgo -installsuffix netgo --ldflags '-extldflags "-static"'  \
      -o /build/example-xds-server \
        cmd/main.go \
      && \
    /build/example-xds-server --version && \
    apk del .build-deps

# ----------------------------------

FROM alpine:3.18

RUN apk add --no-cache tzdata && \
    cp /usr/share/zoneinfo/Asia/Tokyo /etc/localtime

WORKDIR /app
COPY --from=builder /build/   /app/

RUN set -eux && \
    apk add --no-cache ca-certificates curl dumb-init openssl && \
    /app/example-xds-server --version

EXPOSE 8000
EXPOSE 8001
VOLUME [ "/app" ]

COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
ENTRYPOINT [ "docker-entrypoint.sh" ]
