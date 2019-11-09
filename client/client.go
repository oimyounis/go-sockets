package client

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

const DELIMITER = "§"
const DELIMITER_LENGTH = len(DELIMITER)

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
	time.Sleep(time.Millisecond * 5)
}

func (s *Socket) Emit(event, data string) {
	go emit(s, event, data)
}

func (s *Socket) Start() {
	s.connectEvent(s)
	go s.socketReceiver()
}

func (s *Socket) Listen() {
	s.connectEvent(s)
	s.socketReceiver()
}

func (s *Socket) On(event string, callback MessageHandler) {
	s.events[event] = callback
}

func (s *Socket) socketReceiver() {
	sockBuffer := bufio.NewReader(s.connection)
	for {
		message, err := sockBuffer.ReadString('\n')
		if err != nil {
			// log.Println(err)
			break
		}

		go func() {
			if strings.Contains(message, DELIMITER) {
				delimIdx := strings.Index(message, DELIMITER)
				event := message[:delimIdx]
				if handler, ok := s.events[event]; ok {
					data := message[delimIdx+DELIMITER_LENGTH:]
					if strings.Contains(data, "\n") {
						data = strings.ReplaceAll(data, "\\n", "\n")
					}
					go handler(data)
				}
			}
		}()
	}
	s.connection.Close()
	s.disconnectEvent(s)
}

func (s *Socket) OnConnect(handler ConnectionHandler) {
	s.connectEvent = handler
}

func (s *Socket) OnDisconnect(handler ConnectionHandler) {
	s.disconnectEvent = handler
}

func emit(socket *Socket, event, data string) {
	time.Sleep(time.Millisecond * 10)
	if strings.Contains(data, "\n") {
		data = strings.ReplaceAll(data, "\n", "\\n")
	}
	socket.connection.Write([]byte(fmt.Sprintf("%v%v%v\n", event, DELIMITER, data)))
	// fmt.Fprintf(socket.connection, fmt.Sprintf("%v%v%v\n", event, DELIMITER, data))
}

func New(address string) *Socket {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Printf("Couldn't connect to server: %v", err)
	}

	return &Socket{connection: conn, events: map[string]MessageHandler{}, connectEvent: func(socket *Socket) {}, disconnectEvent: func(socket *Socket) {}}
}
