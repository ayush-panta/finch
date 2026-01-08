//go:build darwin

package bridgecredhelper

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

type CredentialServer struct {
	listener net.Listener
}

var server *CredentialServer

func StartCredentialServer(finchRootPath string) error {
	fmt.Fprintf(os.Stderr, "[CREDS] 1. Starting server\n")
	if server != nil {
		fmt.Fprintf(os.Stderr, "[CREDS] 2. Server already exists, returning\n")
		return nil
	}
	
	fmt.Fprintf(os.Stderr, "[CREDS] 3. Creating socket path\n")
	socketPath := filepath.Join(finchRootPath, "lima", "data", "finch", "sock", "creds.sock")
	os.MkdirAll(filepath.Dir(socketPath), 0750)
	os.Remove(socketPath)
	
	fmt.Fprintf(os.Stderr, "[CREDS] 4. Creating listener\n")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[CREDS] 5. Listen failed: %v\n", err)
		return err
	}
	os.Chmod(socketPath, 0600)
	
	fmt.Fprintf(os.Stderr, "[CREDS] 6. Creating server struct\n")
	server = &CredentialServer{listener: listener}
	fmt.Fprintf(os.Stderr, "[CREDS] 7. Server struct created, server=%p\n", server)
	
	fmt.Fprintf(os.Stderr, "[CREDS] 8. Creating channel\n")
	started := make(chan bool)
	
	fmt.Fprintf(os.Stderr, "[CREDS] 9. Starting goroutine\n")
	go func() {
		fmt.Fprintf(os.Stderr, "[CREDS] 10. Inside goroutine\n")
		fmt.Fprintf(os.Stderr, "[CREDS] 11. Goroutine server=%p\n", server)
		if server == nil {
			fmt.Fprintf(os.Stderr, "[CREDS] 12. ERROR: server is nil in goroutine!\n")
			return
		}
		fmt.Fprintf(os.Stderr, "[CREDS] 13. Signaling started\n")
		started <- true
		fmt.Fprintf(os.Stderr, "[CREDS] 14. About to call server.serve()\n")
		server.serve()
		fmt.Fprintf(os.Stderr, "[CREDS] 15. server.serve() returned (should never see this)\n")
	}()
	
	fmt.Fprintf(os.Stderr, "[CREDS] 16. Waiting for started signal\n")
	<-started
	fmt.Fprintf(os.Stderr, "[CREDS] 17. Got started signal, returning\n")
	return nil
}

func (s *CredentialServer) serve() {
	fmt.Fprintf(os.Stderr, "[CREDS] 18. serve() method called, s=%p\n", s)
	if s == nil {
		fmt.Fprintf(os.Stderr, "[CREDS] 19. ERROR: serve() receiver is nil!\n")
		return
	}
	if s.listener == nil {
		fmt.Fprintf(os.Stderr, "[CREDS] 20. ERROR: listener is nil!\n")
		return
	}
	fmt.Fprintf(os.Stderr, "[CREDS] 21. Starting Accept() loop\n")
	for {
		fmt.Fprintf(os.Stderr, "[CREDS] 22. About to call Accept()...\n")
		
		// Add a small delay to see if this helps with timing
		conn, err := s.listener.Accept()
		fmt.Fprintf(os.Stderr, "[CREDS] 22.5. Accept() returned, err=%v\n", err)
		
		if err != nil {
			fmt.Fprintf(os.Stderr, "[CREDS] 23. Accept error: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "[CREDS] 24. Got connection!\n")
		go handle(conn)
	}
}

func StopCredentialServer() {
	if server != nil {
		server.listener.Close()
		server = nil
	}
}



func handle(conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "[CREDS] PANIC in handle(): %v\n", r)
		}
		conn.Close()
	}()
	
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[CREDS] Read error: %v\n", err)
		return
	}
	
	req := string(buf[:n])
	fmt.Fprintf(os.Stderr, "[CREDS] Request: %q\n", req)
	
	lines := strings.Split(strings.TrimSpace(req), "\n")
	if len(lines) < 2 || lines[0] != "get" {
		fmt.Fprintf(os.Stderr, "[CREDS] Invalid request\n")
		return
	}
	
	creds, err := callCredentialHelper("get", lines[1], "", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[CREDS] Credential helper error: %v\n", err)
		creds = &dockerCredential{ServerURL: lines[1]}
	}
	
	data, _ := json.Marshal(creds)
	conn.Write(data)
	fmt.Fprintf(os.Stderr, "[CREDS] Response sent\n")
}
