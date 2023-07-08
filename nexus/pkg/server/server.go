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
}

func tcpServer(ctx context.Context, portFile string, shutdownChan chan bool) {
	addr := "127.0.0.1:0"
	listen, err := net.Listen("tcp", addr)
	if err != nil {
		LogError(slog.Default(), "cant listen", err)
	}

	defer func() {
		err := listen.Close()
		if err != nil {
			LogError(slog.Default(), "Error closing listener:", err)
		}
	}()

	slog.Info(fmt.Sprintf("Server is running on: %v", addr))
	port := listen.Addr().(*net.TCPAddr).Port
	slog.Info(fmt.Sprintf("PORT %v", port))

	writePortFile(portFile, port)

	slog.Info("Nexus server started")

	// Run a separate goroutine to handle incoming connections
	wg := &sync.WaitGroup{}
	shutdown := false
	go func() {
		for {
			conn, err := listen.Accept()
			if err != nil {
				if shutdown {
					slog.Debug("Server is shutting down")
					break
				}
				LogError(slog.Default(), "Failed to accept conn.", err)
				continue
			}
			slog.Info("Accepted connection")
			wg.Add(1)
			go func() {
				defer wg.Done()
				handleConnection(ctx, conn, shutdownChan)
			}()
			slog.Debug("handleConnection: DONE")
		}
	}()

	shutdown = <-shutdownChan
	slog.Debug("tcpServer: waiting for connections to finish")
	wg.Wait()
	slog.Info("Nexus server stopped")
}

func handleConnection(ctx context.Context, conn net.Conn, shutdownChan chan<- bool) {
	connection := NewConnection(ctx, conn, shutdownChan)
	slog.Debug("handleConnection: NewConnection")
	connection.start()
	slog.Debug("handleConnection: DONE")
}

func WandbService(portFilename string) {
	shutdownChan := make(chan bool)
	defer close(shutdownChan)
	ctx := context.Background()
	tcpServer(ctx, portFilename, shutdownChan)
}
