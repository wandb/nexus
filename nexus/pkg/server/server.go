package server

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/wandb/wandb/nexus/pkg/service"
	"google.golang.org/protobuf/proto"

	"golang.org/x/exp/slog"
)

const BufferSize = 32

// Server is the nexus server
type Server struct {
	// ctx is the context for the server
	ctx context.Context

	// listener is the underlying listener
	listener net.Listener

	// wg is the WaitGroup for the server
	wg sync.WaitGroup

	// teardownChan is the channel for signaling and waiting for teardown
	teardownChan chan struct{}

	// shutdownChan is the channel for signaling shutdown
	shutdownChan chan struct{}
}

// NewServer creates a new server
func NewServer(ctx context.Context, addr string, portFile string) *Server {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("can not listen", "error", err)
		// TODO: handle error
	}

	s := &Server{
		ctx:          ctx,
		listener:     listener,
		wg:           sync.WaitGroup{},
		teardownChan: make(chan struct{}),
		shutdownChan: make(chan struct{}),
	}

	port := s.listener.Addr().(*net.TCPAddr).Port
	writePortFile(portFile, port)
	// set env var for address of server
	err = os.Setenv("WANDB_NEXUS_ADDR", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		slog.Error("can not set env var", "error", err)
	}
	fmt.Printf("server addr: %s\n", s.listener.Addr().String())

	s.wg.Add(1)
	go s.Serve()
	return s
}

func (s *Server) GetAddr() *net.TCPAddr {
	return s.listener.Addr().(*net.TCPAddr)
}

// Serve serves the server
func (s *Server) Serve() {
	defer s.wg.Done()
	slog.Info("server is running", "addr", s.listener.Addr())
	// Run a separate goroutine to handle incoming connections
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.shutdownChan:
				slog.Debug("server shutting down...")
				return
			default:
				slog.Error("failed to accept conn.", "error", err)
			}
		} else {
			s.wg.Add(1)
			go func() {
				s.handleConnection(s.ctx, conn)
				s.wg.Done()
			}()
		}
	}
}

// Close closes the server
func (s *Server) Close() {
	<-s.teardownChan
	close(s.shutdownChan)
	if err := s.listener.Close(); err != nil {
		slog.Error("failed to Close listener", err)
	}
	s.wg.Wait()
	slog.Info("server is closed")
}

// handleConnection handles a single connection
func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	nc := NewConnection(ctx, conn, s.teardownChan)

	scanner := bufio.NewScanner(conn)
	tokenizer := &Tokenizer{}
	scanner.Split(tokenizer.Split)
	for scanner.Scan() {
		msg := &service.ServerRequest{}
		if err := proto.Unmarshal(scanner.Bytes(), msg); err != nil {
			slog.Error(
				"unmarshalling error",
				"err", err,
				"conn", conn.RemoteAddr())
		} else {
			slog.Debug("received message", "msg", msg, "conn", conn.RemoteAddr())
			nc.inChan <- msg
		}
	}
	close(nc.inChan)
}
