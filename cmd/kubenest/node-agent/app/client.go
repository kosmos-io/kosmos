package app

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
)

var (
	ClientCmd = &cobra.Command{
		Use:   "client",
		Short: "A WebSocket client CLI tool to execute commands and file uploads",
	}
	shCmd = &cobra.Command{
		Use:     "sh [command]",
		Short:   "Execute a command via WebSocket",
		RunE:    cmdCmdRun,
		Example: `node-agent client sh -u=[user] -p=[pass] -a="127.0.0.1:5678" -o ls -r "-l"`,
	}
	uploadCmd = &cobra.Command{
		Use:     "upload",
		Short:   "Upload a file via WebSocket",
		RunE:    cmdUploadRun,
		Example: `node-agent upload -u=[user] -p=[pass] -a="127.0.0.1:5678" -f /tmp -n=app.go`,
	}
	ttyCmd = &cobra.Command{
		Use:   "tty",
		Short: "Execute a command via WebSocket with TTY",
		RunE:  cmdTtyRun,
	}
	wg sync.WaitGroup
)
var uniqueValuesMap = make(map[string]bool)
var dialer = websocket.DefaultDialer

func init() {
	dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	ClientCmd.PersistentFlags().StringSliceVarP(&wsAddr, "addr", "a", []string{}, "WebSocket address (e.g., host1:port1,host2:port2)")
	ClientCmd.MarkPersistentFlagRequired("addr")

	// PreRunE check param
	ClientCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		for _, value := range wsAddr {
			if _, exists := uniqueValuesMap[value]; exists {
				return errors.New("duplicate values are not allowed")
			}
			uniqueValuesMap[value] = true
		}
		return nil
	}

	shCmd.Flags().StringArrayVarP(&params, "param", "r", []string{}, "Command parameters")
	shCmd.Flags().StringVarP(&operation, "operation", "o", "", "Operation to perform")
	shCmd.MarkFlagRequired("addr")

	uploadCmd.Flags().StringVarP(&fileName, "name", "n", "", "Name of the file to upload")
	uploadCmd.Flags().StringVarP(&filePath, "path", "f", "", "Path to the file to upload")
	uploadCmd.MarkFlagRequired("name")
	uploadCmd.MarkFlagRequired("path")

	ttyCmd.Flags().StringVarP(&operation, "operation", "o", "", "Operation to perform")

	ClientCmd.AddCommand(shCmd)
	ClientCmd.AddCommand(uploadCmd)
	ClientCmd.AddCommand(ttyCmd)
}

func cmdTtyRun(cmd *cobra.Command, args []string) error {
	headers := http.Header{
		"Authorization": {"Basic " + basicAuth(user, password)},
	}
	cmdStr := fmt.Sprintf("command=%s", operation)
	// execute one every wsAddr
	for _, addr := range wsAddr {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			wsURL := fmt.Sprintf("wss://%s/tty/?%s", addr, cmdStr)
			fmt.Println("Executing tty:", cmdStr, "on", addr)
			err := connectTty(wsURL, headers)
			if err != nil {
				log.Errorf("failed to execute command: %v on %s: %v\n", err, addr, cmdStr)
			}
		}(addr)
	}
	wg.Wait()
	return nil
}

func connectTty(wsURL string, headers http.Header) error {
	ws, resp, err := dialer.Dial(wsURL, headers)
	defer wsRespClose(resp)
	if err != nil {
		return fmt.Errorf("WebSocket dial error: %v", err)
	}
	defer ws.Close()
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	inputChan := make(chan string)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			inputChan <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			log.Fatalf("Scanner error: %v", err)
		}
	}()
	done := make(chan struct{})
	// Read messages from the WebSocket server
	go func() {
		defer close(done)
		for {
			_, message, err := ws.ReadMessage()
			if err != nil {
				log.Infof("ReadMessage: %v", err)
				interrupt <- os.Interrupt
				return
			}
			fmt.Printf("%s", message)
		}
	}()
	// Main event loop
	for {
		select {
		case msg := <-inputChan:
			// Send user input to the WebSocket server
			if err := ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("%s\n", msg))); err != nil {
				log.Infof("WriteMessage: %v", err)
				return err
			}
			if msg == "exit" {
				return nil
			}
		case <-interrupt:
			// Cleanly close the connection on interrupt
			log.Infof("Interrupt received, closing connection...")
			if err := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
				log.Infof("CloseMessage: %v", err)
				return err
			}
			select {
			case <-done:
			}
			return nil
		}
	}
}

func cmdCmdRun(cmd *cobra.Command, args []string) error {
	if len(operation) == 0 {
		log.Errorf("operation is required")
		return fmt.Errorf("operation is required")
	}
	if len(user) == 0 || len(password) == 0 {
		log.Errorf("user and password are required")
		return fmt.Errorf("user and password are required")
	}
	// use set to remove duplicate for wsAddr
	return executeWebSocketCommand()
}

func cmdUploadRun(cmd *cobra.Command, args []string) error {
	return uploadFile(filePath, fileName)
}

func executeWebSocketCommand() error {
	headers := http.Header{
		"Authorization": {"Basic " + basicAuth(user, password)},
	}
	cmdStr := fmt.Sprintf("command=%s", operation)
	// Build params part of the URL
	if len(params) > 1 {
		paramsStr := "args="
		for _, param := range params {
			paramsStr += param + "&&args="
		}
		paramsStr = paramsStr[:len(paramsStr)-7]
		cmdStr = fmt.Sprintf("command=%s&&%s", operation, paramsStr)
	}

	// execute one every wsAddr
	for _, addr := range wsAddr {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			wsURL := fmt.Sprintf("wss://%s/cmd/?%s", addr, cmdStr)
			fmt.Println("Executing command:", cmdStr, "on", addr)
			err := connectAndHandleMessages(wsURL, headers)
			if err != nil {
				log.Errorf("failed to execute command: %v on %s: %v\n", err, addr, cmdStr)
			}
		}(addr)
	}
	wg.Wait()
	return nil
}

func uploadFile(filePath, fileName string) error {
	headers := http.Header{
		"Authorization": {"Basic " + basicAuth(user, password)},
	}
	for _, addr := range wsAddr {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			wsURL := fmt.Sprintf("wss://%s/upload/?file_name=%s&file_path=%s", addr, url.QueryEscape(filepath.Base(fileName)), url.QueryEscape(filePath))
			fmt.Println("Uploading file:", fileName, "from", filePath, "to", addr)
			err := connectAndSendFile(wsURL, headers, filePath, fileName)
			if err != nil {
				log.Errorf("failed to upload file: %v on %s: %v\n", err, addr, fileName)
			}
		}(addr)
	}
	wg.Wait()
	return nil
}

func wsRespClose(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}

func connectAndHandleMessages(wsURL string, headers http.Header) error {
	ws, resp, err := dialer.Dial(wsURL, headers)
	defer wsRespClose(resp)
	if err != nil {
		return fmt.Errorf("WebSocket dial error: %v", err)
	}
	defer ws.Close()

	handleMessages(ws)
	return nil
}

func connectAndSendFile(wsURL string, headers http.Header, filePath, fileName string) error {
	ws, resp, err := dialer.Dial(wsURL, headers)
	if err != nil {
		return fmt.Errorf("WebSocket dial error: %v", err)
	}
	defer wsRespClose(resp)
	defer ws.Close()

	sendFile(ws, fileName)

	handleMessages(ws)
	return nil
}

func basicAuth(user, password string) string {
	auth := user + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func handleMessages(ws *websocket.Conn) {
	defer ws.Close()
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			log.Println("Read message error:", err)
			return
		}
		fmt.Printf("Received message: %s\n", message)
	}
}

func sendFile(ws *websocket.Conn, filePath string) {
	//if file not exists, close connection
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Errorf("File not exists: %v", err)
		err := ws.WriteMessage(websocket.BinaryMessage, []byte("EOF"))
		if err != nil {
			log.Printf("Write message error: %v", err)
		}
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		log.Errorf("File open error: %v", err)
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
			log.Errorf("failed to read file %v:", err)
			return
		}
		dataToSend := buffer[:n]

		_ = ws.WriteMessage(websocket.BinaryMessage, dataToSend)
	}

	err = ws.WriteMessage(websocket.BinaryMessage, []byte("EOF"))
	log.Infof("send EOF ----")
	if err != nil {
		log.Errorf("Write message error: %v", err)
	}
}
