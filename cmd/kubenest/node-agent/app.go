// main.go
package main

import (
	"bufio"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var (
	addr     = flag.String("addr", ":5678", "websocket service address")
	certFile = flag.String("cert", "cert.pem", "SSL certificate file")
	keyFile  = flag.String("key", "key.pem", "SSL key file")
	user     = flag.String("user", "", "Username for authentication")
	password = flag.String("password", "", "Password for authentication")
	log      = logrus.New()
)

var upgrader = websocket.Upgrader{} // use default options

func init() {
	log.Out = os.Stdout

	file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.Out = file
	} else {
		log.Info("Failed to log to file, using default stderr")
	}
	log.SetLevel(logrus.InfoLevel)
}
func main() {
	flag.Parse()
	if *user == "" {
		_user := os.Getenv("WEB_USER")
		if _user != "" {
			*user = _user
		}
	}

	if *password == "" {
		_password := os.Getenv("WEB_PASS")
		if _password != "" {
			*password = _password
		}
	}
	if len(*user) == 0 || len(*password) == 0 {
		flag.Usage()
		log.Errorf("-user and -password are required %s %s", *user, *password)
		return
	}
	start(*addr, *certFile, *keyFile, *user, *password)
}

// start server
func start(addr, certFile, keyFile, user, password string) {
	passwordHash := sha256.Sum256([]byte(password))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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

	log.Errorf("failed to start server %v", server.ListenAndServeTLS("", ""))
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
