package client

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

type ConnectionHandler func(socket *Socket)
type MessageHandler func(data string)

type Socket struct {
	connection      net.Conn
	events          map[string]MessageHandler
	connectEvent    ConnectionHandler
	disconnectEvent ConnectionHandler
}

func (s *Socket) EmitSync(event, data string) {
	emit(s, event, data)
}

func (s *Socket) Emit(event, data string) {
	go emit(s, event, data)
}

func (s *Socket) Start() {
	go s.socketReceiver()
}

func (s *Socket) Listen() {
	s.socketReceiver()
}

func (s *Socket) On(event string, callback MessageHandler) {
	s.events[event] = callback
}

func (s *Socket) socketReceiver() {
	for {
		message, err := bufio.NewReader(s.connection).ReadString('\n')
		if err != nil {
			// log.Println("Disconnected from server")
			break
		}

		if strings.Contains(message, "§") {
			delimIdx := strings.Index(message, "§")
			event := message[:delimIdx]
			if handler, ok := s.events[event]; ok {
				data := message[delimIdx+2:]
				if strings.Contains(data, "\n") {
					data = strings.ReplaceAll(data, "\\n", "\n")
				}
				go handler(data)
			}
		}
	}
	s.connection.Close()
	s.disconnectEvent(s)
}

func emit(socket *Socket, event, data string) {
	if strings.Contains(data, "\n") {
		data = strings.ReplaceAll(data, "\n", "\\n")
	}
	log.Println("out: " + fmt.Sprintf("%v§%v\n", event, data))
	fmt.Fprintf(socket.connection, fmt.Sprintf("%v§%v\n", event, data))
	time.Sleep(time.Millisecond * 10)
}

func New(address string) *Socket {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Printf("Couldn't connect to server: %v", err)
	}

	return &Socket{connection: conn, events: map[string]MessageHandler{}}
}

func NewWithEvents(address string, onConnect ConnectionHandler, onDisconnect ConnectionHandler) *Socket {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Printf("Couldn't connect to server: %v", err)
	}

	socket := &Socket{connection: conn, events: map[string]MessageHandler{}, connectEvent: onConnect, disconnectEvent: onDisconnect}
	onConnect(socket)
	return socket
}
