# nocalhost remote debug shell for clusterlink/proxy
export G0111MODULE=on;
export GOPROXY='https://goproxy.cn,direct';
export GOSUMDB=off;
dlv --headless --log --listen :9009 --api-version 2 --accept-multiclient debug /home/nocalhost-dev/cmd/clusterlink/proxy/main.go -- --enable-pprof=true