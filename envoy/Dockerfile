# envoy GLIBC_2.14-GLIBC_2.27
FROM ubuntu:20.04

COPY --from=envoyproxy/envoy:v1.28.1 /usr/local/bin/envoy /usr/local/bin

WORKDIR /envoy
ADD ./envoy.yaml /etc/envoy/

RUN set -eux && \
    echo "dash dash/sh boolean false" | debconf-set-selections && \
    DEBIAN_FRONTEND=noninteractive dpkg-reconfigure dash

RUN apt-get update -y && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends tzdata

ENV TZ Asia/Tokyo
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

# set default tz
RUN set -eux && \
    apt-get install -y ca-certificates curl openssl dumb-init && \
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
