package exector

import (
	"fmt"
	"strings"

	"github.com/gorilla/websocket"

	env "github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/env"
)

type CMDExector struct {
	Cmd string
}

func AddPrefix(cmd string) string {
	cmdAbsolutePaths := env.GetCMDPaths()
	if len(cmdAbsolutePaths) == 0 {
		return cmd
	}
	for _, cmdAbsolutePath := range cmdAbsolutePaths {
		if strings.HasSuffix(cmdAbsolutePath, fmt.Sprintf("/%s", cmd)) {
			return cmdAbsolutePath
		}
	}
	return cmd
}

func (e *CMDExector) GetWebSocketOption() WebSocketOption {
	cmdArgs := strings.Split(e.Cmd, " ")
	command := cmdArgs[0]
	rawQuery := "command=" + AddPrefix(command)
	if len(cmdArgs) > 1 {
		args := cmdArgs[1:]
		rawQuery = rawQuery + "&args=" + strings.Join(args, "&args=")
	}
	return WebSocketOption{
		Path:     "cmd/",
		RawQuery: rawQuery,
	}
}

func (e *CMDExector) SendHandler(_ *websocket.Conn, _ <-chan struct{}, _ chan struct{}, _ *ExectorReturn) {
}
