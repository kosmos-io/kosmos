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
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var (
	addr     = flag.String("addr", ":5678", "websocket service address")
	certFile = flag.String("cert", "cert.pem", "SSL certificate file")
	keyFile  = flag.String("key", "key.pem", "SSL key file")
	user     = flag.String("user", "", "Username for authentication")
	password = flag.String("password", "", "Password for authentication")
)

var upgrader = websocket.Upgrader{} // use default options

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if len(*user) == 0 || len(*password) == 0 {
		*user = os.Getenv("WEB_USER")
		*password = os.Getenv("WEB_PASS")
		if len(*user) == 0 || len(*password) == 0 {
			flag.Usage()
			log.Fatal("-user and -password are required")
		}
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
			log.Print("upgrade:", err)
			return
		}
		defer conn.Close()

		u, err := url.Parse(r.RequestURI)
		if err != nil {
			log.Print("parse uri:", err)
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

	log.Printf("Starting server on %s", addr)
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	tlsConfig.Certificates = make([]tls.Certificate, 1)
	tlsConfig.Certificates[0], _ = tls.LoadX509KeyPair(certFile, keyFile)
	server := &http.Server{
		Addr:              addr,
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Fatal(server.ListenAndServeTLS("", ""))
}

func handleUpload(conn *websocket.Conn, params url.Values) {
	fileName := params.Get("file_name")
	filePath := params.Get("file_path")
	log.Printf("Uploading file name %s, file path %s", fileName, filePath)
	defer conn.Close()
	if len(fileName) != 0 && len(filePath) != 0 {
		// mkdir
		err := os.MkdirAll(filePath, 0775)
		if err != nil {
			log.Print("mkdir:", err)
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, fmt.Sprintf("failed to make directory: %v", err)))
			return
		}
		file := filepath.Join(filePath, fileName)
		// check if the file already exists
		if _, err := os.Stat(file); err == nil {
			log.Printf("File %s already exists", file)
			timestamp := time.Now().Format("2006-01-02-150405000")
			bakFilePath := fmt.Sprintf("%s_%s_bak", file, timestamp)
			err = os.Rename(file, bakFilePath)
			if err != nil {
				log.Printf("failed to rename file: %v", err)
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, fmt.Sprintf("failed to rename file: %v", err)))
				return
			}
		}
		// create file with append
		fp, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("failed to open file: %v", err)
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, fmt.Sprintf("failed to open file: %v", err)))
			return
		}
		defer fp.Close()
		// receive data from websocket
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				log.Printf("failed to read message : %s", err)
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, fmt.Sprintf("failed to read message: %v", err)))
				return
			}
			// check if the file end
			if string(data) == "EOF" {
				log.Printf("finish file data transfer %s", file)
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, fmt.Sprintf("finish file data transfer: %v", "EOF")))
				return
			}
			// data to file
			_, err = fp.Write(data)
			if err != nil {
				log.Printf("failed to write data to file : %s", err)
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, fmt.Sprintf("failed write data to file: %v", err)))
				return
			}
		}
	}
	_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInvalidFramePayloadData, "Invalid file_name or file_path"))
}

func handleCmd(conn *websocket.Conn, params url.Values) {
	command := params.Get("command")
	if command == "" {
		log.Printf("No command specified %v", params)
		_ = conn.WriteMessage(websocket.TextMessage, []byte("No command specified"))
		return
	}

	cmd := exec.Command("sh", "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("failed to execute command : %v", err)
		_ = conn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
	} else {
		_ = conn.WriteMessage(websocket.TextMessage, out)
	}
	exitCode := cmd.ProcessState.ExitCode()
	log.Printf("Command finished with exit code %d", exitCode)
	_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, fmt.Sprintf("Exit Code: %d", exitCode)))
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
		log.Printf("Error creating temporary file: %v", err)
		return
	}
	defer os.Remove(tempFile.Name()) // Clean up temporary file
	defer tempFile.Close()
	tempFilefp, err := os.OpenFile(tempFile.Name(), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Error opening temporary file: %v", err)
	}
	for {
		// Read message from WebSocket client
		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("failed to read message : %s", err)
			break
		}
		if string(data) == "EOF" {
			log.Printf("finish file data transfer %s", tempFile.Name())
			break
		}

		// Write received data to the temporary file
		if _, err := tempFilefp.Write(data); err != nil {
			log.Printf("Error writing data to temporary file: %v", err)
			continue
		}
	}

	// Execute the Python script
	executeCmd := append(command, tempFile.Name())
	executeCmd = append(executeCmd, args...)
	// #nosec G204
	cmd := exec.Command(executeCmd[0], executeCmd[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("Error obtaining command output pipe: %v", err)
	}
	defer stdout.Close()

	if err := cmd.Start(); err != nil {
		log.Printf("Error starting command: %v", err)
	}

	// processOutput
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			data := scanner.Bytes()
			log.Printf("%s", data)
			_ = conn.WriteMessage(websocket.TextMessage, data)
		}
	}()

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			log.Printf("Command exited with non-zero status: %v", exitError)
		}
	}
	exitCode := cmd.ProcessState.ExitCode()
	_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, fmt.Sprintf("Exit Code: %d", exitCode)))
}
