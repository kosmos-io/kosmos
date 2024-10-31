package serve

import (
	"bufio"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/kosmos.io/kosmos/cmd/kubenest/node-agent/app/logger"
)

var (
	ServeCmd = &cobra.Command{
		Use:   "serve",
		Short: "Start a WebSocket server",
		RunE:  serveCmdRun,
	}

	certFile string // SSL certificate file
	keyFile  string // SSL key file
	addr     string // server listen address
	log      = logger.GetLogger()
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
} // use default options

func init() {
	// setup flags
	ServeCmd.PersistentFlags().StringVarP(&addr, "addr", "a", ":5678", "websocket service address")
	ServeCmd.PersistentFlags().StringVarP(&certFile, "cert", "c", "cert.pem", "SSL certificate file")
	ServeCmd.PersistentFlags().StringVarP(&keyFile, "key", "k", "key.pem", "SSL key file")
}

func serveCmdRun(_ *cobra.Command, _ []string) error {
	user := viper.GetString("WEB_USER")
	password := viper.GetString("WEB_PASS")
        port := viper.GetString("WEB_PORT")
	if len(user) == 0 || len(password) == 0 {
		log.Errorf("-user and -password are required %s %s", user, password)
		return errors.New("-user and -password are required")
	}
	if port != "" {
		addr = ":" + port
	}
	return Start(addr, certFile, keyFile, user, password)
}

// start server
func Start(addr, certFile, keyFile, user, password string) error {
	passwordHash := sha256.Sum256([]byte(password))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			w.WriteHeader(http.StatusOK)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userPassBase64 := strings.TrimPrefix(auth, "Basic ")
		userPassBytes, err := base64.StdEncoding.DecodeString(userPassBase64)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userPass := strings.SplitN(string(userPassBytes), ":", 2)
		if len(userPass) != 2 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userHash := sha256.Sum256([]byte(userPass[1]))
		if userPass[0] != user || userHash != passwordHash {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Errorf("http upgrade to websocket failed : %v", err)
			return
		}
		defer conn.Close()

		u, err := url.Parse(r.RequestURI)
		if err != nil {
			log.Errorf("parse uri: %s, %v", r.RequestURI, err)
			return
		}
		queryParams := u.Query()

		switch {
		case strings.HasPrefix(r.URL.Path, "/upload"):
			handleUpload(conn, queryParams)
		case strings.HasPrefix(r.URL.Path, "/cmd"):
			handleCmd(conn, queryParams)
		case strings.HasPrefix(r.URL.Path, "/py"):
			handleScript(conn, queryParams, []string{"python3", "-u"})
		case strings.HasPrefix(r.URL.Path, "/sh"):
			handleScript(conn, queryParams, []string{"sh"})
		case strings.HasPrefix(r.URL.Path, "/tty"):
			handleTty(conn, queryParams)
		case strings.HasPrefix(r.URL.Path, "/check"):
			handleCheck(conn, queryParams)
		default:
			_ = conn.WriteMessage(websocket.TextMessage, []byte("Invalid path"))
		}
	})

	log.Infof("Starting server on %s", addr)
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
	}
	tlsConfig.Certificates = make([]tls.Certificate, 1)
	tlsConfig.Certificates[0], _ = tls.LoadX509KeyPair(certFile, keyFile)
	server := &http.Server{
		Addr:              addr,
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: 10 * time.Second,
	}
	err := server.ListenAndServeTLS("", "")
	if err != nil {
		log.Errorf("failed to start server %v", err)
	}
	return err
}

func handleCheck(conn *websocket.Conn, params url.Values) {
	port := params.Get("port")
	if len(port) == 0 {
		log.Errorf("port is required")
		return
	}
	log.Infof("Check port %s", port)
	address := fmt.Sprintf(":%s", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Infof("port not avalible %s %v", address, err)
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, fmt.Sprintf("%d", 1)))
		return
	}
	defer listener.Close()
	log.Infof("port avalible %s", address)
	// _ = conn.WriteMessage(websocket.BinaryMessage, []byte("0"))
	_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, fmt.Sprintf("%d", 0)))
}

func handleTty(conn *websocket.Conn, queryParams url.Values) {
	entrypoint := queryParams.Get("command")
	if len(entrypoint) == 0 {
		log.Errorf("command is required")
		return
	}
	log.Infof("Executing command %s", entrypoint)
	cmd := exec.Command(entrypoint)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Errorf("failed to start command %v", err)
		return
	}
	defer func() {
		_ = ptmx.Close()
	}()
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				log.Errorf("error resizing pty: %s", err)
			}
		}
	}()
	ch <- syscall.SIGWINCH                        // Initial resize.
	defer func() { signal.Stop(ch); close(ch) }() // Cleanup signals when done.
	done := make(chan struct{})
	// Use a goroutine to copy PTY output to WebSocket
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				log.Errorf("PTY read error: %v", err)
				break
			}
			log.Printf("Received message: %s", buf[:n])
			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				log.Errorf("WebSocket write error: %v", err)
				break
			}
		}
		done <- struct{}{}
	}()
	// echo off
	//ptmx.Write([]byte("stty -echo\n"))
	// Set stdin in raw mode.
	//oldState, err := term.MakeRaw(int(ptmx.Fd()))
	//if err != nil {
	//	panic(err)
	//}
	//defer func() { _ = term.Restore(int(ptmx.Fd()), oldState) }() // Best effort.

	// Disable Bracketed Paste Mode in bash shell
	//	_, err = ptmx.Write([]byte("printf '\\e[?2004l'\n"))
	//	if err != nil {
	//		log.Fatal(err)
	//	}

	// Use a goroutine to copy WebSocket input to PTY
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("read from websocket failed: %v, %s", err, string(message))
				break
			}
			log.Printf("Received message: %s", message)    // Debugging line
			if _, err := ptmx.Write(message); err != nil { // Ensure newline character for commands
				log.Printf("PTY write error: %v", err)
				break
			}
		}
		// Signal the done channel when this goroutine finishes
		done <- struct{}{}
	}()

	// Wait for the done channel to be closed
	<-done
}

func handleUpload(conn *websocket.Conn, params url.Values) {
	fileName := params.Get("file_name")
	filePath := params.Get("file_path")
	log.Infof("Uploading file name %s, file path %s", fileName, filePath)
	defer conn.Close()
	if len(fileName) != 0 && len(filePath) != 0 {
		// mkdir
		err := os.MkdirAll(filePath, 0775)
		if err != nil {
			log.Errorf("mkdir: %s %v", filePath, err)
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, fmt.Sprintf("failed to make directory: %v", err)))
			return
		}
		file := filepath.Join(filePath, fileName)
		// check if the file already exists
		if _, err := os.Stat(file); err == nil {
			log.Infof("File %s already exists", file)
			timestamp := time.Now().Format("2006-01-02-150405000")
			bakFilePath := fmt.Sprintf("%s_%s_bak", file, timestamp)
			err = os.Rename(file, bakFilePath)
			if err != nil {
				log.Errorf("failed to rename file: %v", err)
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, fmt.Sprintf("failed to rename file: %v", err)))
				return
			}
		}
		// create file with append
		fp, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Errorf("failed to open file: %v", err)
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, fmt.Sprintf("failed to open file: %v", err)))
			return
		}
		defer fp.Close()
		// receive data from websocket
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				log.Errorf("failed to read message : %s", err)
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, fmt.Sprintf("failed to read message: %v", err)))
				return
			}
			// check if the file end
			if string(data) == "EOF" {
				log.Infof("finish file data transfer %s", file)
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, fmt.Sprintf("%d", 0)))
				return
			}
			// data to file
			_, err = fp.Write(data)
			if err != nil {
				log.Errorf("failed to write data to file : %s", err)
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, fmt.Sprintf("failed write data to file: %v", err)))
				return
			}
		}
	}
	_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInvalidFramePayloadData, "Invalid file_name or file_path"))
}

/*
0 → success
non-zero → failure
Exit code 1 indicates a general failure
Exit code 2 indicates incorrect use of shell builtins
Exit codes 3-124 indicate some error in job (check software exit codes)
Exit code 125 indicates out of memory
Exit code 126 indicates command cannot execute
Exit code 127 indicates command not found
Exit code 128 indicates invalid argument to exit
Exit codes 129-192 indicate jobs terminated by Linux signals
For these, subtract 128 from the number and match to signal code
Enter kill -l to list signal codes
Enter man signal for more information
*/
func handleCmd(conn *websocket.Conn, params url.Values) {
	command := params.Get("command")
	args := params["args"]
	// if the command is file, the file should have execute permission
	if command == "" {
		log.Warnf("No command specified %v", params)
		_ = conn.WriteMessage(websocket.TextMessage, []byte("No command specified"))
		return
	}
	execCmd(conn, command, args)
}

func handleScript(conn *websocket.Conn, params url.Values, command []string) {
	defer conn.Close()
	args := params["args"]
	if len(args) == 0 {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("No command specified"))
	}
	// Write data to a temporary file
	tempFile, err := os.CreateTemp("", "script_*")
	if err != nil {
		log.Errorf("Error creating temporary file: %v", err)
		return
	}
	defer os.Remove(tempFile.Name()) // Clean up temporary file
	defer tempFile.Close()
	tempFilefp, err := os.OpenFile(tempFile.Name(), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Errorf("Error opening temporary file: %v", err)
	}
	for {
		// Read message from WebSocket client
		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Errorf("failed to read message : %s", err)
			break
		}
		if string(data) == "EOF" {
			log.Infof("finish file data transfer %s", tempFile.Name())
			break
		}

		// Write received data to the temporary file
		if _, err := tempFilefp.Write(data); err != nil {
			log.Errorf("Error writing data to temporary file: %v", err)
			continue
		}
	}
	executeCmd := append(command, tempFile.Name())
	executeCmd = append(executeCmd, args...)
	// Execute the Python script
	execCmd(conn, executeCmd[0], executeCmd[1:])
}

func execCmd(conn *websocket.Conn, command string, args []string) {
	// #nosec G204
	cmd := exec.Command(command, args...)
	log.Infof("Executing command: %s, %v", command, args)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Warnf("Error obtaining command output pipe: %v", err)
	}
	defer stdout.Close()

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Warnf("Error obtaining command error pipe: %v", err)
	}
	defer stderr.Close()

	// Channel for signaling command completion
	doneCh := make(chan struct{})
	defer close(doneCh)
	// processOutput
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			data := scanner.Bytes()
			log.Warnf("%s", data)
			_ = conn.WriteMessage(websocket.TextMessage, data)
		}
		scanner = bufio.NewScanner(stderr)
		for scanner.Scan() {
			data := scanner.Bytes()
			log.Warnf("%s", data)
			_ = conn.WriteMessage(websocket.TextMessage, data)
		}
		doneCh <- struct{}{}
	}()
	if err := cmd.Start(); err != nil {
		errStr := strings.ToLower(err.Error())
		log.Warnf("Error starting command: %v, %s", err, errStr)
		_ = conn.WriteMessage(websocket.TextMessage, []byte(errStr))
		if strings.Contains(errStr, "no such file") {
			exitCode := 127
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, fmt.Sprintf("%d", exitCode)))
		}
	}

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			log.Warnf("Command : %s exited with non-zero status: %v", command, exitError)
		}
	}
	<-doneCh
	exitCode := cmd.ProcessState.ExitCode()
	log.Infof("Command : %s finished with exit code %d", command, exitCode)
	_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, fmt.Sprintf("%d", exitCode)))
}
