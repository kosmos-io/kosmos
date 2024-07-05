package client

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/kosmos.io/kosmos/cmd/kubenest/node-agent/app/logger"
)

var (
	log       = logger.GetLogger()
	ClientCmd = &cobra.Command{
		Use:   "client",
		Short: "A WebSocket client CLI tool to execute commands and file uploads",
		Long:  "support execute remote command, upload file and pty",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	shCmd = &cobra.Command{
		Use:     "sh [command]",
		Short:   "Execute a command via WebSocket",
		Long:    "Execute command on remote server",
		RunE:    cmdCmdRun,
		Example: `node-agent client sh -u=[user] -p=[pass] -a="127.0.0.1:5678" -o ls -r "-l"`,
	}
	uploadCmd = &cobra.Command{
		Use:     "upload",
		Short:   "Upload a file via WebSocket",
		Long:    "upload file to remote servers",
		RunE:    cmdUploadRun,
		Example: `node-agent upload -u=[user] -p=[pass] -a="127.0.0.1:5678" -f /tmp -n=app.go`,
	}
	ttyCmd = &cobra.Command{
		Use:   "tty",
		Short: "Execute a command via WebSocket with TTY",
		Long:  "execute command on remote server use pyt",
		RunE:  cmdTtyRun,
	}
	wg sync.WaitGroup

	wsAddr    []string // websocket client connect address list
	filePath  string   // the server path to save upload file
	fileName  string   // local file to upload
	params    []string // New slice to hold multiple command parameters
	operation string   // operation for client to execute
)
var uniqueValuesMap = make(map[string]bool)
var dialer = websocket.DefaultDialer

func BasicAuth(user, password string) string {
	auth := user + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
func init() {
	// #nosec G402
	dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	ClientCmd.PersistentFlags().StringSliceVarP(&wsAddr, "addr", "a", []string{}, "WebSocket address (e.g., host1:port1,host2:port2)")
	err := ClientCmd.MarkPersistentFlagRequired("addr")
	if err != nil {
		return
	}

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
	_ = shCmd.MarkFlagRequired("addr")

	uploadCmd.Flags().StringVarP(&fileName, "name", "n", "", "Name of the file to upload")
	uploadCmd.Flags().StringVarP(&filePath, "path", "f", "", "Path to the file to upload")
	// avoid can't show subcommand help and execute subcommand
	_ = uploadCmd.MarkFlagRequired("name")
	_ = uploadCmd.MarkFlagRequired("path")

	ttyCmd.Flags().StringVarP(&operation, "operation", "o", "", "Operation to perform")
	err = ttyCmd.MarkFlagRequired("operation") // Ensure 'operation' flag is required for ttyCmd
	if err != nil {
		return
	}
	ClientCmd.AddCommand(shCmd)
	ClientCmd.AddCommand(uploadCmd)
	ClientCmd.AddCommand(ttyCmd)
}

func cmdTtyRun(cmd *cobra.Command, args []string) error {
	auth, err := getAuth(cmd)
	if err != nil {
		return err
	}
	headers := http.Header{
		"Authorization": {"Basic " + auth},
	}
	cmdStr := fmt.Sprintf("command=%s", operation)
	// execute one every wsAddr
	for _, addr := range wsAddr {
		wsURL := fmt.Sprintf("wss://%s/tty/?%s", addr, cmdStr)
		fmt.Println("Executing tty:", cmdStr, "on", addr)
		err := connectTty(wsURL, headers)
		if err != nil {
			log.Errorf("failed to execute command: %v on %s: %v\n", err, addr, cmdStr)
		}
	}
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
	// set raw for control char
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw terminal: %v", err)
	}
	defer func(fd int, oldState *term.State) {
		err := term.Restore(fd, oldState)
		if err != nil {
			log.Errorf("failed to restore terminal: %v", err)
		}
	}(int(os.Stdin.Fd()), oldState)

	inputChan := make(chan []byte)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				log.Println("Read input error:", err)
				return
			}
			inputChan <- buf[0:n]
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
	go func() {
		<-interrupt
		// Cleanly close the connection on interrupt
		log.Infof("Interrupt received, closing connection...")
		if err := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
			log.Infof("CloseMessage: %v", err)
			return
		}
	}()

	for {
		select {
		case msg, ok := <-inputChan:
			if !ok {
				return nil
			}
			// Send user input to the WebSocket server
			if err := ws.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				log.Infof("WriteMessage: %v", err)
				return err
			}
			if bytes.Equal(msg, []byte("exit")) {
				return nil
			}
		case <-done:
			return nil
		}
	}
}

func cmdCmdRun(cmd *cobra.Command, args []string) error {
	if len(operation) == 0 {
		log.Errorf("operation is required")
		return fmt.Errorf("operation is required")
	}
	auth, err := getAuth(cmd)
	if err != nil {
		return err
	}
	// use set to remove duplicate for wsAddr
	return executeWebSocketCommand(auth)
}

func cmdUploadRun(cmd *cobra.Command, args []string) error {
	auth, err := getAuth(cmd)
	if err != nil {
		return err
	}
	return uploadFile(filePath, fileName, auth)
}

func getAuth(cmd *cobra.Command) (string, error) {
	user, _ := cmd.Flags().GetString("user")
	password, _ := cmd.Flags().GetString("password")
	if len(user) == 0 || len(password) == 0 {
		log.Errorf("user and password are required")
		return "", fmt.Errorf("user and password are required")
	}
	auth := BasicAuth(user, password)
	return auth, nil
}

func executeWebSocketCommand(auth string) error {
	headers := http.Header{
		"Authorization": {"Basic " + auth},
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

func uploadFile(filePath, fileName, auth string) error {
	headers := http.Header{
		"Authorization": {"Basic " + auth},
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
