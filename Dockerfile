FROM debian:stretch as builderc

# Fluent Bit version
ENV FLB_MAJOR 1
ENV FLB_MINOR 3
ENV FLB_PATCH 7
ENV FLB_VERSION 1.3.7

ENV DEBIAN_FRONTEND noninteractive

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      build-essential \
      cmake \
      make \
      wget \
      unzip \
      libssl1.0-dev \
      libsasl2-dev \
      pkg-config \
      libsystemd-dev \
      zlib1g-dev \
      flex \
      bison

RUN mkdir -p /fluent-bit/bin /fluent-bit/etc /fluent-bit/log /tmp/src/
COPY . /tmp/src/
RUN rm -rf /tmp/src/build/*

WORKDIR /tmp/src/build/
RUN cmake -DFLB_DEBUG=Off \
          -DFLB_TRACE=Off \
          -DFLB_JEMALLOC=On \
          -DFLB_BUFFERING=On \
          -DFLB_TLS=On \
          -DFLB_SHARED_LIB=Off \
          -DFLB_EXAMPLES=Off \
          -DFLB_HTTP_SERVER=On \
          -DFLB_IN_SYSTEMD=On \
          -DFLB_OUT_KAFKA=On ..

RUN make -j $(getconf _NPROCESSORS_ONLN)
RUN install bin/fluent-bit /fluent-bit/bin/fluent-bit-core

# Configuration files
COPY conf/fluent-bit.conf \
     conf/parsers.conf \
     conf/parsers_java.conf \
     conf/parsers_extra.conf \
     conf/parsers_openstack.conf \
     conf/parsers_cinder.conf \
     conf/plugins.conf \
     /fluent-bit/etc/

FROM golang:1.13.6-alpine3.11 as buildergo
RUN mkdir -p /fluent-bit
COPY main.go go.mod /fluent-bit/
WORKDIR /fluent-bit
RUN CGO_ENABLED=0 go build -o fluent-bit main.go

FROM gcr.io/distroless/cc
MAINTAINER KubeSphere <kubesphere@yunify.com>
LABEL Description="Fluent Bit docker image" Vendor="KubeSphere Organization" Version="1.0"

COPY --from=builderc /usr/lib/x86_64-linux-gnu/*sasl* /usr/lib/x86_64-linux-gnu/
COPY --from=builderc /usr/lib/x86_64-linux-gnu/libz* /usr/lib/x86_64-linux-gnu/
COPY --from=builderc /lib/x86_64-linux-gnu/libz* /lib/x86_64-linux-gnu/
COPY --from=builderc /usr/lib/x86_64-linux-gnu/libssl.so* /usr/lib/x86_64-linux-gnu/
COPY --from=builderc /usr/lib/x86_64-linux-gnu/libcrypto.so* /usr/lib/x86_64-linux-gnu/
# These below are all needed for systemd
COPY --from=builderc /lib/x86_64-linux-gnu/libsystemd* /lib/x86_64-linux-gnu/
COPY --from=builderc /lib/x86_64-linux-gnu/libselinux.so* /lib/x86_64-linux-gnu/
COPY --from=builderc /lib/x86_64-linux-gnu/liblzma.so* /lib/x86_64-linux-gnu/
COPY --from=builderc /usr/lib/x86_64-linux-gnu/liblz4.so* /usr/lib/x86_64-linux-gnu/
COPY --from=builderc /lib/x86_64-linux-gnu/libgcrypt.so* /lib/x86_64-linux-gnu/
COPY --from=builderc /lib/x86_64-linux-gnu/libpcre.so* /lib/x86_64-linux-gnu/
COPY --from=builderc /lib/x86_64-linux-gnu/libgpg-error.so* /lib/x86_64-linux-gnu/

COPY --from=builderc /fluent-bit /fluent-bit
COPY --from=buildergo /fluent-bit/fluent-bit /fluent-bit/bin/fluent-bit

#
EXPOSE 2020

# Entry point
CMD ["/fluent-bit/bin/fluent-bit", "-c", "/fluent-bit/etc/fluent-bit.conf"]