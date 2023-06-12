package server

import (
	"context"
	"fmt"
	"net"
	"os"

	log "github.com/sirupsen/logrus"
)

func writePortFile(portFile string, port int) {
	tempFile := fmt.Sprintf("%s.tmp", portFile)
	f, err := os.Create(tempFile)
	checkError(err)
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	_, err = f.WriteString(fmt.Sprintf("sock=%d\n", port))
	checkError(err)

	_, err = f.WriteString("EOF")
	checkError(err)

	err = f.Sync()
	checkError(err)

	err = os.Rename(tempFile, portFile)
	checkError(err)
}

type NexusServer struct {
	teardownChan chan bool
	listen       net.Listener
}

func (ns *NexusServer) handleConnection(conn net.Conn) {
	defer func(conn net.Conn) {
		log.Info("Closing connection with ", conn.RemoteAddr())
		err := conn.Close()
		if err != nil {
			log.Error("Error closing connection:", err)
			return
		}
	}(conn)

	ctx, cancel := context.WithCancel(context.Background())

	connection := NewConnection(ctx, cancel, conn, ns.teardownChan)
	connection.handle()
}

func (ns *NexusServer) teardown() {
	streamManager.Close()
}

func tcpServer(portFile string) {
	addr := "127.0.0.1:0"
	listen, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer func(listen net.Listener) {
		_ = listen.Close()
	}(listen)

	server := NexusServer{listen: listen}

	log.Println("Server is running on:", addr)
	port := listen.Addr().(*net.TCPAddr).Port
	log.Println("PORT", port)

	writePortFile(portFile, port)

	for {
		fmt.Println("Waiting for connection")
		select {
		case <-server.teardownChan:
			// Handle teardown request
			log.Println("Server teardown requested")
			fmt.Println("Server teardown requested")
			server.teardown()
			return
		default:
			// contents of server.teardownChan
			fmt.Println(server.teardownChan)
			conn, err := listen.Accept()
			if err != nil {
				log.Println("Failed to accept conn.", err)
				continue
			}

			go server.handleConnection(conn)
		}
	}
}

func WandbService(portFilename string) {
	tcpServer(portFilename)
}
