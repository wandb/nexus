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
	shutdown bool
	listen   net.Listener
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
		conn, err := listen.Accept()
		if err != nil {
			if server.shutdown {
				log.Println("shutting down...")
				break
			}
			log.Println("Failed to accept conn.", err)
			continue
		}

		ctx, cancel := context.WithCancel(context.Background())

		go handleConnection(ctx, cancel, conn, &server)
	}
}

func WandbService(portFilename string) {
	tcpServer(portFilename)
}
