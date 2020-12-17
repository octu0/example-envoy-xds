# envoy GLIBC_2.14-GLIBC_2.27
FROM ubuntu:18.04

# using envoy v1.15.3
COPY --from=envoyproxy/envoy:v1.15.3 /usr/local/bin/envoy /usr/local/bin

WORKDIR /envoy
ADD ./envoy.yaml /etc/envoy/

ENV TZ Asia/Tokyo

# set default tz
RUN set -eux && \
    apt-get clean && \
    mv /var/lib/apt/lists /tmp && \
    mkdir -p /var/lib/apt/lists/partial && \
    apt-get clean && \
    apt-get update -y && \
    apt-get install -y ca-certificates curl gosu openssl dumb-init tzdata && \
    cp /usr/share/zoneinfo/Asia/Tokyo /etc/localtime && \
    envoy --version && \
    apt-get purge -y && \
    apt-get autoclean

RUN addgroup envoy && \
    adduser --system envoy && \
    adduser envoy envoy && \
    groupmems -g envoy -l

EXPOSE 8080
VOLUME [ "/envoy" ]

COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
ENTRYPOINT [ "docker-entrypoint.sh" ]
