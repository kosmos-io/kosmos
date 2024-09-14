FROM ubuntu:latest AS release-env

ARG BINARY

WORKDIR /app
# copy install file to container
# build context is _output/xx/xx
COPY agent/* ./

# install rsync
RUN apt-get update && apt-get install -y rsync pwgen openssl && \
    openssl req -x509 -sha256 -new -nodes -days 3650 -newkey rsa:2048 -keyout key.pem -out cert.pem -subj "/C=CN/O=Kosmos/OU=Kosmos/CN=kosmos.io"

COPY ${BINARY} /app

# install command
CMD ["bash", "/app/install.sh", "/app"]
