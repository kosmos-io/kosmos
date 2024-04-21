package exector

import (
	"bufio"
	"fmt"
	"os"

	"github.com/gorilla/websocket"
	"k8s.io/klog/v2"
)

type SCPExector struct {
	DstFilePath string
	DstFileName string
	SrcFile     string
	SrcByte     []byte
}

func (e *SCPExector) GetWebSocketOption() WebSocketOption {
	return WebSocketOption{
		Path:     "upload/",
		RawQuery: fmt.Sprintf("file_name=%s&&file_path=%s", e.DstFileName, e.DstFilePath),
	}
}

func (e *SCPExector) SendHandler(conn *websocket.Conn, done <-chan struct{}, interrupt chan struct{}, result *ExectorReturn) {
	errHandler := func(err error) {
		klog.V(4).Infof("write: %s", err)
		result.Reason = err.Error()
		close(interrupt)
	}

	send := func(data []byte) error {
		err := conn.WriteMessage(websocket.BinaryMessage, []byte(data))
		if err != nil {
			return err
		}
		return nil
	}

	if len(e.SrcByte) > 0 {
		if err := send(e.SrcByte); err != nil {
			errHandler(err)
			return
		}
	} else {
		file, err := os.Open(e.SrcFile)
		if err != nil {
			errHandler(err)
			return
		}
		defer file.Close()

		// 指定每次读取的数据块大小
		bufferSize := 1024 // 例如每次读取 1024 字节
		buffer := make([]byte, bufferSize)

		reader := bufio.NewReader(file)
		for {
			select {
			case <-interrupt:
				return
			case <-done:
				return
			default:
			}
			n, err := reader.Read(buffer)
			if err != nil {
				// check if EOF
				if err.Error() == "EOF" {
					break
				}
				errHandler(err)
				return
			}
			dataToSend := buffer[:n]

			if err := send(dataToSend); err != nil {
				errHandler(err)
				return
			}
		}
	}

	if err := send([]byte("EOF")); err != nil {
		errHandler(err)
		return
	}
}
