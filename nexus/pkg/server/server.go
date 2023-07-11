package server

import (
	"bufio"
	"context"
	"fmt"
	"github.com/wandb/wandb/nexus/pkg/service"
	"golang.org/x/exp/slog"
	"google.golang.org/protobuf/proto"
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
	teardownChan chan struct{}
	shutdownChan chan struct{}
	wg           sync.WaitGroup
}

func NewServer(ctx context.Context, addr string, portFile string) *Server {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		LogError(slog.Default(), "cant listen", err)
	}

	s := &Server{
		listener:     listener,
		teardownChan: make(chan struct{}),
		shutdownChan: make(chan struct{}),
		wg:           sync.WaitGroup{},
	}

	port := s.listener.Addr().(*net.TCPAddr).Port
	writePortFile(portFile, port)

	s.wg.Add(1)
	go s.serve(ctx)
	slog.Info("server is running", "addr", addr)
	return s
}

func (s *Server) serve(ctx context.Context) {
	defer s.wg.Done()

	slog.Debug("server started")
	// Run a separate goroutine to handle incoming connections
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.shutdownChan:
				slog.Debug("server shutting down...")
				return
			default:
				LogError(slog.Default(), "failed to accept conn.", err)
			}
		} else {
			slog.Info("accepted connection", "addr", conn.RemoteAddr())
			s.wg.Add(1)
			go func() {
				s.handleConnection(ctx, conn)
				s.wg.Done()
			}()
		}
	}
}

func (s *Server) Close() {
	<-s.teardownChan
	close(s.shutdownChan)
	if err := s.listener.Close(); err != nil {
		slog.Error("failed to close listener", err)
	}
	s.wg.Wait()
	slog.Info("server is closed")
}

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	nexusConn := NewConnection(ctx, conn, s.teardownChan)

	defer close(nexusConn.inChan)

	scanner := bufio.NewScanner(conn)
	tokenizer := &Tokenizer{}
	scanner.Split(tokenizer.split)
	for scanner.Scan() {
		msg := &service.ServerRequest{}
		if err := proto.Unmarshal(scanner.Bytes(), msg); err != nil {
			slog.Error(
				"unmarshalling error",
				slog.String("err", err.Error()),
				slog.String("conn", conn.RemoteAddr().String()))
		} else {
			slog.Debug("received message", "msg", msg.String(), "conn", conn.RemoteAddr().String())
			nexusConn.inChan <- msg
		}
	}
}
