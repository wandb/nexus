package server

import (
	"context"
	"fmt"
	"golang.org/x/exp/slog"
	"net"
	"os"
	"sync"
)

func writePortFile(portFile string, port int) {
	tempFile := fmt.Sprintf("%s.tmp", portFile)
	f, err := os.Create(tempFile)
	if err != nil {
		LogError(slog.Default(), "fail create", err)
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	if _, err = f.WriteString(fmt.Sprintf("sock=%d\n", port)); err != nil {
		LogError(slog.Default(), "fail write", err)
	}

	if _, err = f.WriteString("EOF"); err != nil {
		LogError(slog.Default(), "fail write EOF", err)
	}

	if err = f.Sync(); err != nil {
		LogError(slog.Default(), "fail sync", err)
	}

	if err = os.Rename(tempFile, portFile); err != nil {
		LogError(slog.Default(), "fail rename", err)
	}
	slog.Info(fmt.Sprintf("PORT %v", port))
}

type Server struct {
	listener     net.Listener
	shutdownChan chan struct{}
	shutdown     bool
	wg           sync.WaitGroup
}

func NewServer(addr string, portFile string) *Server {
	s := &Server{
		shutdownChan: make(chan struct{}),
		wg:           sync.WaitGroup{},
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		LogError(slog.Default(), "cant listen", err)
	}
	s.listener = listener

	slog.Info(fmt.Sprintf("Server is running on: %v", addr))
	port := s.listener.Addr().(*net.TCPAddr).Port
	writePortFile(portFile, port)

	s.wg.Add(1)
	ctx := context.Background()
	go s.serve(ctx)
	return s
}

func (s *Server) serve(ctx context.Context) {
	defer s.wg.Done()

	slog.Info("Nexus server started")

	// Run a separate goroutine to handle incoming connections
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.shutdown {
				slog.Info("Server shutdown")
				return
			}
			LogError(slog.Default(), "Failed to accept conn.", err)
		} else {
			slog.Info("Accepted connection %v", conn.RemoteAddr())
			s.wg.Add(1)
			go func() {
				handleConnection(ctx, conn, s.shutdownChan)
				s.wg.Done()
			}()
		}
	}
}

func (s *Server) close() {
	<-s.shutdownChan
	s.shutdown = true
	err := s.listener.Close()
	if err != nil {
		slog.Error("Failed to close listener", err)
	}
	s.wg.Wait()
}

func handleConnection(ctx context.Context, conn net.Conn, shutdownChan chan struct{}) {
	connection := NewConnection(ctx, conn, shutdownChan)
	slog.Debug("handleConnection: NewConnection")
	connection.start()
	slog.Debug("handleConnection: done	")
}

func WandbService(portFilename string) {
	server := NewServer("127.0.0.1:0", portFilename)
	server.close()
}
