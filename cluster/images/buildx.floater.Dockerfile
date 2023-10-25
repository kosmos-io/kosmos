FROM alpine:3.17.1

ARG BINARY
ARG TARGETPLATFORM

RUN apk add --no-cache ca-certificates
RUN apk update && apk upgrade
RUN apk add ip6tables iptables curl

COPY ${TARGETPLATFORM}/${BINARY} /bin/${BINARY}

RUN adduser -D -g clusterlink -u 1002 clusterlink && \
    chown -R clusterlink:clusterlink /bin/${BINARY} && \
    chmod u+s /bin/ping

USER clusterlink
