package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"

	log "github.com/sirupsen/logrus"
)

func InitLogging() {
	logFile, err := os.OpenFile("/tmp/logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}

	logToConsole := false
	if logToConsole {
		mw := io.MultiWriter(os.Stderr, logFile)
		log.SetOutput(mw)
	} else {
		log.SetOutput(logFile)
	}

	log.SetFormatter(&log.JSONFormatter{})
	log.SetLevel(log.DebugLevel)
}

func writePortfile(portfile string, port int) {
	tmpfile := fmt.Sprintf("%s.tmp", portfile)
	f, err := os.Create(tmpfile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if _, err = f.WriteString(fmt.Sprintf("sock=%d\n", port)); err != nil {
		log.Fatal(err)
	}
	if _, err = f.WriteString("EOF"); err != nil {
		log.Fatal(err)
	}
	if err = f.Sync(); err != nil {
		log.Fatal(err)
	}

	if err = os.Rename(tmpfile, portfile); err != nil {
		log.Fatal(err)
	}
}

type NexusServer struct {
	shutdown bool
	listen   net.Listener
}

func tcpServer(portfile string) {
	addr := "127.0.0.1:0"
	listen, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer listen.Close()

	serverState := NexusServer{listen: listen}

	log.Println("Server is running on:", addr)
	port := listen.Addr().(*net.TCPAddr).Port
	log.Println("PORT", port)

	writePortfile(portfile, port)

	for {
		conn, err := listen.Accept()
		if err != nil {
			if serverState.shutdown {
				log.Println("shutting down...")
				break
			}
			log.Println("Failed to accept conn.", err)
			continue
		}

		go handleConnection(context.Background(), &serverState, conn)
	}
}

func wbService(portFile string) {
	tcpServer(portFile)
}

func WandbService(portFilename string) {
	wbService(portFilename)
}

type Service interface {
	Serve()
}

type TcpService struct {
	portFile string
}

func (s *TcpService) Serve() {
	tcpServer(s.portFile)
}
