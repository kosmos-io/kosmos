package main

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// create dialer
var dialer = *websocket.DefaultDialer

// test addr user pass
var testAddr, username, pass string
var headers http.Header

var currentDir, _ = os.Getwd()
var parentDir string

func init() {
	// #nosec G402
	dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	_, filename, _, _ := runtime.Caller(0)
	currentDir = filepath.Dir(filename)
	parentDir = filepath.Dir(currentDir)
	_ = os.Setenv("WEB_USER", "")
	_ = os.Setenv("WEB_PASS", "")
	username = os.Getenv("WEB_USER")
	pass = os.Getenv("WEB_PASS")
	testAddr = "127.0.0.1:5678"
	headers = http.Header{
		"Authorization": {"Basic " + basicAuth(username, pass)},
	}
	go start(":5678", "cert.pem", "key.pem", username, pass)
	time.Sleep(10 * time.Second)
}
func wsRespClose(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}
func TestCmd(t *testing.T) {
	fmt.Println("Command test")
	command := url.QueryEscape("ls -l")
	ws, resp, err := dialer.Dial("wss://"+testAddr+"/cmd/?command="+command, headers)
	defer wsRespClose(resp)
	if err != nil {
		log.Printf("Dial error: %v (HTTP response: %v)", err, resp)
		return
	}
	defer ws.Close()

	handleMessages(ws)
}

func TestUpload(t *testing.T) {
	fmt.Println("Upload file test")
	fileName := url.QueryEscape("app.go")
	filePath := url.QueryEscape("/tmp/websocket")

	ws, resp, err := dialer.Dial("wss://"+testAddr+"/upload/?file_name="+fileName+"&file_path="+filePath, headers)
	if err != nil {
		log.Printf("Dial error: %v (HTTP response: %v)", err, resp)
		return
	}
	defer wsRespClose(resp)
	defer ws.Close()

	sendFile(ws, filepath.Join(currentDir, "app.go"))
	handleMessages(ws)
}

func TestShellScript(t *testing.T) {
	fmt.Println("Shell script test")

	ws, resp, err := dialer.Dial("wss://"+testAddr+"/sh/?args=10&&args=10", headers)
	if err != nil {
		log.Printf("Dial error: %v (HTTP response: %v)", err, resp)
		return
	}
	defer wsRespClose(resp)
	defer ws.Close()

	sendFile(ws, filepath.Join(parentDir, "count.sh"))
	handleMessages(ws)
}

func TestPyScript(t *testing.T) {
	fmt.Println("Python script test")
	ws, resp, err := dialer.Dial("wss://"+testAddr+"/py/?args=10&&args=10", headers)
	if err != nil {
		log.Printf("Dial error: %v (HTTP response: %v)", err, resp)
		return
	}
	defer wsRespClose(resp)
	defer ws.Close()
	sendFile(ws, filepath.Join(parentDir, "count.py"))
	handleMessages(ws)
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func handleMessages(ws *websocket.Conn) {
	defer ws.Close()
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			log.Println("Read message end :", err)
			return
		}
		fmt.Printf("Received message: %s\n", message)
	}
}

func sendFile(ws *websocket.Conn, filePath string) {
	//if file not exists, close connection
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("File not exists: %v", err)
		err := ws.WriteMessage(websocket.BinaryMessage, []byte("EOF"))
		if err != nil {
			log.Printf("Write message error: %v", err)
		}
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("File open error: %v", err)
	}
	defer file.Close()
	// 指定每次读取的数据块大小
	bufferSize := 1024 // 例如每次读取 1024 字节
	buffer := make([]byte, bufferSize)

	reader := bufio.NewReader(file)
	for {
		n, err := reader.Read(buffer)
		if err != nil {
			// check if EOF
			if err.Error() == "EOF" {
				break
			}
			log.Printf("failed to read file %v:", err)
			return
		}
		dataToSend := buffer[:n]

		_ = ws.WriteMessage(websocket.BinaryMessage, dataToSend)
	}

	err = ws.WriteMessage(websocket.BinaryMessage, []byte("EOF"))
	log.Printf("send EOF ----")
	if err != nil {
		log.Printf("Write message error: %v", err)
	}
}
