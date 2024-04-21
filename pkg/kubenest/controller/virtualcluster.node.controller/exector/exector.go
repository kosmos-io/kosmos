// Copyright 2015 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package exector

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/klog/v2"
)

type Status int

const (
	SUCCESS Status = iota
	FAILED
)

type ExectorReturn struct {
	Status  Status
	Reason  string
	LastLog string
}

func (r *ExectorReturn) String() string {
	return fmt.Sprintf("%d, %s, %s", r.Status, r.Reason, r.LastLog)
}

type Exector interface {
	GetWebSocketOption() WebSocketOption
	SendHandler(conn *websocket.Conn, done <-chan struct{}, interrupt chan struct{}, result *ExectorReturn)
}

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
	// TODO:
	if strings.Contains(ret.LastLog, "No such file or directory") {
		// try to update shell script
		shellPath := os.Getenv("EXECTOR_SHELL_PATH")
		if len(shellPath) == 0 {
			shellPath = "."
		}
		srcFile := fmt.Sprintf("%s/kubelet_node_helper.sh", shellPath)

		klog.V(4).Infof("exector: src file path %s", srcFile)

		scpExector := &SCPExector{
			DstFilePath: ".",
			DstFileName: "kubelet_node_helper.sh",
			SrcFile:     srcFile,
		}

		if ret := h.DoExectorReal(stopCh, scpExector); ret.Status == SUCCESS {
			return h.DoExectorReal(stopCh, exector)
		} else {
			return ret
		}
	}
	return ret
}

func (h *ExectorHelper) DoExectorReal(stopCh <-chan struct{}, exector Exector) *ExectorReturn {
	// default is error
	result := &ExectorReturn{
		FAILED, "init exector return status", "",
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
			if err != nil {
				klog.V(4).Infof("read: %s", err)
				cerr, ok := err.(*websocket.CloseError)
				if ok && cerr.Text == "0" {
					result.Status = SUCCESS
					result.Reason = "success"
				} else {
					result.Reason = err.Error()
				}
				return
			}
			klog.V(4).Infof("recv: %s", string(message))
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
		exectorPort = os.Getenv("EXECTOR_SERVER_PORT")
		if len(exectorPort) == 0 {
			exectorPort = "5678"
		}
	} else {
		exectorPort = port
	}

	token := os.Getenv("EXECTOR_SHELL_TOKEN")
	if len(token) == 0 {
		// token example
		// const username = "xxxxxxxx"
		// const password = "xxxxxxxx"
		// token = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
		// nolint
		token = "YWRtaW46YmljaF9vb3NoMnpvaDZPaA=="
	}

	return &ExectorHelper{
		Token: token,
		Addr:  fmt.Sprintf("%s:%s", addr, exectorPort),
	}
}
