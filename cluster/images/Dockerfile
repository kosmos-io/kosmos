#Dockerfile：定义镜像的构建步骤，包括基础镜像、工作目录、依赖安装和启动命令。
#docker build：根据Dockerfile构建镜像，-t参数指定镜像的名称和标签。

FROM alpine:3.17.1

ARG BINARY

RUN apk add --no-cache ca-certificates
RUN apk update && apk upgrade
RUN apk add ip6tables iptables curl tcpdump busybox-extras

COPY ${BINARY} /bin/${BINARY}
