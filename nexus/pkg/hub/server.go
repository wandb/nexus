package hub

import (
	"context"
	//"context"
	log "github.com/sirupsen/logrus"
	"net"
)

type Service interface {
	Serve()
}

type TcpService struct {
}

func (s *TcpService) Serve() {
	port := "1337"
	addr := "127.0.0.1:" + port

	listen, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer func(listen net.Listener) {
		err := listen.Close()
		if err != nil {
			log.Println("Error closing listener:", err)
			return
		}
	}(listen)

	log.Println("Server is running on:", addr)

	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Println("Failed to accept conn.", err)
			continue
		}

		log.Println("Accepted connection from:", conn.RemoteAddr())

		go handleConnection(context.TODO(), conn)
	}
}
