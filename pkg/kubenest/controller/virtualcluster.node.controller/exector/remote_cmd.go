package exector

import (
	"strings"

	"github.com/gorilla/websocket"
)

type CMDExector struct {
	Cmd string
}

func (e *CMDExector) GetWebSocketOption() WebSocketOption {
	cmdArgs := strings.Split(e.Cmd, " ")
	command := cmdArgs[0]
	rawQuery := "command=" + command
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
