package exector

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/klog/v2"

	env "github.com/kosmos.io/kosmos/pkg/kubenest/controller/virtualcluster.node.controller/env"
	"github.com/kosmos.io/kosmos/pkg/utils"
)

type Status int

const (
	SUCCESS Status = iota
	FAILED
)

const (
	NotFoundText = "127"
)

// nolint:revive
type ExectorReturn struct {
	Status  Status
	Reason  string
	LastLog string
	Text    string
	Code    int
}

func (r *ExectorReturn) String() string {
	return fmt.Sprintf("%d, %s, %s, %d", r.Status, r.Reason, r.LastLog, r.Code)
}

// nolint:revive
type Exector interface {
	GetWebSocketOption() WebSocketOption
	SendHandler(conn *websocket.Conn, done <-chan struct{}, interrupt chan struct{}, result *ExectorReturn)
}

// nolint:revive
type ExectorHelper struct {
	Token string
	Addr  string
}

func (h *ExectorHelper) createWebsocketConnection(opt WebSocketOption) (*websocket.Conn, *http.Response, error) {
	u := url.URL{Scheme: "wss", Host: h.Addr, Path: opt.Path, RawQuery: url.PathEscape(opt.RawQuery)}
	// nolint
	dl := websocket.Dialer{TLSClientConfig: &tls.Config{RootCAs: nil, InsecureSkipVerify: true}}

	return dl.Dial(u.String(), http.Header{
		"Authorization": []string{"Basic " + h.Token},
	})
}

type WebSocketOption struct {
	Path     string
	Addr     string
	RawQuery string
}

func (h *ExectorHelper) DoExector(stopCh <-chan struct{}, exector Exector) *ExectorReturn {
	ret := h.DoExectorReal(stopCh, exector)
	if ret.Text == NotFoundText {
		// try to update shell script
		srcEnvFile := env.GetExectorShellEnvPath()
		klog.V(4).Infof("exector: src file path %s", srcEnvFile)

		scpEnvExector := &SCPExector{
			DstFilePath: ".",
			DstFileName: env.GetExectorShellEnvName(),
			SrcFile:     srcEnvFile,
		}

		srcShellFile := env.GetExectorShellPath()
		klog.V(4).Infof("exector: src file path %s", srcShellFile)

		scpShellExector := &SCPExector{
			DstFilePath: ".",
			DstFileName: env.GetExectorShellName(),
			SrcFile:     srcShellFile,
		}

		if ret := h.DoExectorReal(stopCh, scpEnvExector); ret.Status == SUCCESS {
			if ret := h.DoExectorReal(stopCh, scpShellExector); ret.Status == SUCCESS {
				return h.DoExectorReal(stopCh, exector)
			}
		} else {
			return ret
		}
	}
	return ret
}

func (h *ExectorHelper) DoExectorReal(stopCh <-chan struct{}, exector Exector) *ExectorReturn {
	// default is error
	result := &ExectorReturn{
		FAILED, "init exector return status", "", "", 0,
	}

	// nolint
	conn, _, err := h.createWebsocketConnection(exector.GetWebSocketOption())
	if err != nil {
		result.Reason = err.Error()
		return result
	}
	defer conn.Close()

	done := make(chan struct{})
	interrupt := make(chan struct{})

	go exector.SendHandler(conn, done, interrupt, result)

	go func() {
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if len(message) > 0 {
				klog.V(4).Infof("recv: %s", string(message))
			}
			if err != nil {
				klog.V(4).Infof("read: %s", err)
				cerr, ok := err.(*websocket.CloseError)
				if ok {
					if cerr.Text == "0" {
						result.Status = SUCCESS
						result.Reason = "success"
					} else if cerr.Text == NotFoundText {
						result.Status = FAILED
						result.Reason = "command not found"
						result.Text = cerr.Text
					}
					result.Code = cerr.Code
				} else {
					result.Reason = err.Error()
				}
				return
			}
			// klog.V(4).Infof("recv: %s", string(message))
			// last
			result.LastLog = result.LastLog + string(message)
		}
	}()

	for {
		select {
		case <-stopCh: // finished circulate when stopCh is closed
			close(interrupt)
		case <-interrupt:
			err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				result.Reason = err.Error()
				return result
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return result
		case <-done:
			return result
		}
	}
}

func NewExectorHelper(addr string, port string) *ExectorHelper {
	var exectorPort string
	if len(port) == 0 {
		exectorPort = env.GetExectorPort()
	} else {
		exectorPort = port
	}

	token := env.GetExectorToken()
	return &ExectorHelper{
		Token: token,
		Addr:  utils.GenerateAddrStr(addr, exectorPort),
	}
}
