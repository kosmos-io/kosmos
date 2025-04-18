package exector

import (
	"github.com/gorilla/websocket"
)

type CheckExector struct {
	Port string
}

func (e *CheckExector) GetWebSocketOption() WebSocketOption {
	rawQuery := "port=" + e.Port
	return WebSocketOption{
		Path:     "check/",
		RawQuery: rawQuery,
	}
}

func (e *CheckExector) SendHandler(_ *websocket.Conn, _ <-chan struct{}, _ chan struct{}, _ *ExectorReturn) {
}
