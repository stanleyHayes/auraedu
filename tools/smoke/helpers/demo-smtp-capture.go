// Command demo-smtp-capture receives synthetic smoke mail only, without forwarding.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"strings"
	"sync"
	"time"
)

var mu sync.Mutex
var messages []string

func main() {
	if len(os.Args) > 1 && os.Args[1] == "request" {
		if err := requestGateway(); err != nil {
			log.Fatal(err)
		}
		return
	}
	listener, err := net.Listen("tcp", ":2525")
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go receive(conn)
		}
	}()
	http.HandleFunc("GET /messages", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(messages)
	})
	log.Fatal((&http.Server{Addr: ":8025", ReadHeaderTimeout: 5 * time.Second}).ListenAndServe())
}
func receive(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	reader := textproto.NewReader(bufio.NewReader(conn))
	fmt.Fprint(conn, "220 local smoke SMTP\r\n")
	for {
		line, err := reader.ReadLine()
		if err != nil {
			return
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			fmt.Fprint(conn, "500 empty command\r\n")
			continue
		}
		verb := strings.ToUpper(fields[0])
		switch verb {
		case "EHLO", "HELO":
			fmt.Fprint(conn, "250 local-smoke\r\n")
		case "MAIL", "RCPT", "RSET", "NOOP":
			fmt.Fprint(conn, "250 OK\r\n")
		case "DATA":
			fmt.Fprint(conn, "354 End with dot\r\n")
			body, err := io.ReadAll(io.LimitReader(reader.DotReader(), 1<<20))
			if err != nil {
				return
			}
			mu.Lock()
			messages = append(messages, string(body))
			mu.Unlock()
			fmt.Fprint(conn, "250 accepted locally\r\n")
		case "QUIT":
			fmt.Fprint(conn, "221 bye\r\n")
			return
		default:
			fmt.Fprint(conn, "502 unsupported\r\n")
		}
	}
}

// requestGateway lets the host smoke driver reach the real gateway without
// attaching its egress-isolated Docker network to a public bridge.
func requestGateway() error {
	path := os.Getenv("SMOKE_HTTP_PATH")
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return fmt.Errorf("local request path is required")
	}
	body := os.Getenv("SMOKE_HTTP_BODY")
	method := http.MethodGet
	if body != "" {
		method = http.MethodPost
	}
	req, err := http.NewRequest(method, "http://127.0.0.1:8080"+path, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", os.Getenv("SMOKE_HTTP_TENANT"))
	req.Header.Set("Idempotency-Key", os.Getenv("SMOKE_HTTP_IDEMPOTENCY"))
	if token := os.Getenv("SMOKE_HTTP_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			log.Print("close local response: ", err)
		}
	}()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
	}{response.StatusCode, string(payload)})
}
