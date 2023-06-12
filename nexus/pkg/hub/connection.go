package hub

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Message struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type Connection struct {
	ctx    context.Context
	cancel context.CancelFunc
	conn   net.Conn
	queue  chan *Message
}

func NewConnection(ctx context.Context, cancel context.CancelFunc, conn net.Conn) *Connection {
	return &Connection{
		ctx:    ctx,
		cancel: cancel,
		conn:   conn,
		queue:  make(chan *Message),
	}
}

func (c *Connection) handleMessage(msg Message) {
	log.Info("Handling message:", msg)

	// sleep for 2 seconds, then put a message on the outgoing message queue
	time.Sleep(2 * time.Second)

	response := Message{
		Type: "response",
		Data: "Got your message of type " + msg.Type,
	}

	c.queue <- &response
}

func (c *Connection) receive(wg *sync.WaitGroup) {
	defer wg.Done()

	// _, cancel := context.WithCancel(c.ctx)

	scanner := bufio.NewScanner(c.conn)

	for scanner.Scan() {
		text := scanner.Text()
		var msg Message

		if err := json.Unmarshal([]byte(text), &msg); err != nil {
			log.Error("Error unmarshalling message:", err)
			continue
			// return
		}

		// does this have to be serial?
		go c.handleMessage(msg)

		if err := scanner.Err(); err != nil {
			log.Error("Error while scanning:", err)
			c.cancel()
			return
		}
	}
	fmt.Println("Done scanning")
	c.cancel()
}

func (c *Connection) transmit(wg *sync.WaitGroup) {
	defer wg.Done()

	// monitor queue for messages to send
	for {
		select {
		case msg := <-c.queue:
			log.Info("Sending message: ", msg)
			bytes, err := json.Marshal(msg)
			if err != nil {
				log.Error("Error marshalling message:", err)
				continue
			}
			_, err = c.conn.Write(bytes)
			if err != nil {
				log.Error("Error writing to connection:", err)
				continue
			}
		case <-c.ctx.Done():
			log.Info("Context done, closing transmit loop")
			return
		}
	}
}

func (c *Connection) handle() {
	log.Info("Handling connection with ", c.conn.RemoteAddr())
	var wg sync.WaitGroup
	wg.Add(2)
	go c.receive(&wg)
	go c.transmit(&wg)
	wg.Wait()
	log.Info("Connection closed")
}

func handleConnection(ctx context.Context, conn net.Conn) {
	defer func(conn net.Conn) {
		log.Info("Closing connection with ", conn.RemoteAddr())
		err := conn.Close()
		if err != nil {
			log.Error("Error closing connection:", err)
			return
		}
	}(conn)

	ctx, cancel := context.WithCancel(ctx)

	connection := NewConnection(ctx, cancel, conn)
	connection.handle()
}
