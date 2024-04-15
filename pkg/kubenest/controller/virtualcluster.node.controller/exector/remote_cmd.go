package exector

import (
	"github.com/gorilla/websocket"
)

type CMDExector struct {
	Cmd string
}

func (e *CMDExector) GetWebSocketOption() WebSocketOption {
	return WebSocketOption{
		Path:     "cmd/",
		RawQuery: "command=" + e.Cmd,
	}
}

func (e *CMDExector) SendHandler(_ *websocket.Conn, _ <-chan struct{}, _ chan struct{}, _ *ExectorReturn) {
}
