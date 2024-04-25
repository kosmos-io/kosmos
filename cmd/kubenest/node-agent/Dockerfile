# use python3.9 for build
FROM python:3.9-slim-buster AS build-env

WORKDIR /app

COPY . .

RUN apt-get update && \
    apt-get install -y --no-install-recommends build-essential gcc && \
    pip install --no-cache-dir -r requirements.txt && \
    apt-get install -y pwgen && \
    WEB_USER=agent && \
    WEB_PASS=$(pwgen -y 12 1) && \
    sed -i "s/{{WEB_USER}}/$WEB_USER/g" agent.env && \
    sed -i "s/{{WEB_PASS}}/$WEB_PASS/g" agent.env


RUN pip install pyinstaller && \
    pyinstaller --onefile app.py

FROM golang:latest AS build-go
WORKDIR /app
COPY go/* .

RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o node-agent app.go

RUN openssl req -x509 -sha256 -new -nodes -days 3650 -newkey rsa:2048 -keyout key.pem -out cert.pem -subj "/C=CN/ST=JiangSu/L=SuZhou/O=Kosmos/OU=Kosmos/CN=kosmos.io"


FROM ubuntu:latest as release-env
WORKDIR /app

# install rsync
RUN apt-get update && apt-get install -y rsync
# copy install file to container
COPY . .
COPY --from=build-env /app/dist/app /app
COPY --from=build-env /app/agent.env /app
COPY --from=build-go /app/node-agent /app
COPY --from=build-go /app/cert.pem /app
COPY --from=build-go /app/key.pem /app

# install command
 CMD ["bash", "/app/install.sh", "/app"]
