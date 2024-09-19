FROM ubuntu:latest AS release-env

ARG BINARY
ARG TARGETPLATFORM

WORKDIR /app
# copy install file to container
COPY ${TARGETPLATFORM}/agent/* ./

# install rsync
RUN apt-get update && apt-get install -y rsync pwgen openssl && \
    openssl req -x509 -sha256 -new -nodes -days 3650 -newkey rsa:2048 -keyout key.pem -out cert.pem -subj "/C=CN/O=Kosmos/OU=Kosmos/CN=kosmos.io"

COPY ${TARGETPLATFORM}/${BINARY} /app

# install command
CMD ["bash", "/app/install.sh", "/app"]
