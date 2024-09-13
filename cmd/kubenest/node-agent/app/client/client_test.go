package client

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/kosmos.io/kosmos/cmd/kubenest/node-agent/app/serve"
)

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
		"Authorization": {"Basic " + BasicAuth(username, pass)},
	}
	go func() {
		err := serve.Start(":5678", "cert.pem", "key.pem", username, pass)
		if err != nil {
			log.Fatal(err)
		}
	}()
	time.Sleep(10 * time.Second)
}

func TestCmd(_ *testing.T) {
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

func TestUpload(_ *testing.T) {
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

func TestShellScript(_ *testing.T) {
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

func TestPyScript(_ *testing.T) {
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
