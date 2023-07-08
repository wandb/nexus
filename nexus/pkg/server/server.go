package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	"golang.org/x/exp/slog"
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
}

type NexusServer struct {
	shutdownChan chan bool
	shutdown     bool
	listen       net.Listener
}

func tcpServer(portFile string) {
	addr := "127.0.0.1:0"
	listen, err := net.Listen("tcp", addr)
	if err != nil {
		LogError(slog.Default(), "cant listen", err)
	}

	server := NexusServer{shutdownChan: make(chan bool), listen: listen}

	defer func() {
		err := listen.Close()
		if err != nil {
			LogError(slog.Default(), "Error closing listener:", err)
		}
		close(server.shutdownChan)
	}()

	slog.Info(fmt.Sprintf("Server is running on: %v", addr))
	port := listen.Addr().(*net.TCPAddr).Port
	slog.Info(fmt.Sprintf("PORT %v", port))

	writePortFile(portFile, port)

	wg := sync.WaitGroup{}

	// Run a separate goroutine to handle incoming connections
	go func() {
		for {
			conn, err := listen.Accept()
			if err != nil {
				if server.shutdown {
					break // Break when shutdown has been requested
				}
				LogError(slog.Default(), "Failed to accept conn.", err)
				continue
			}

			ctx, cancel := context.WithCancel(context.Background())

			wg.Add(1)
			go func() {
				defer wg.Done()
				handleConnection(ctx, cancel, conn, server.shutdownChan)
			}()
		}
	}()

	// Wait for a shutdown signal
	<-server.shutdownChan
	server.shutdown = true
	slog.Debug("Shutting down server...")
	wg.Wait()
	slog.Debug("Server shutdown complete")
}

func handleConnection(ctx context.Context, cancel context.CancelFunc, conn net.Conn, shutdownChan chan<- bool) {
	connection := NewConnection(ctx, cancel, conn, shutdownChan)
	slog.Debug("handleConnection: NewConnection")

	defer connection.close()

	connection.start()
	slog.Debug("handleConnection: DONE")
}

func WandbService(portFilename string) {
	tcpServer(portFilename)
}
