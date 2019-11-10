package server

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const DELIMITER = "§"
const DELIMITER_LENGTH = len(DELIMITER)

var delimiterRegex *regexp.Regexp = regexp.MustCompile(`(.*[^\\])§((.*\n?.*)*)`)

type ConnectionHandler func(socket *Socket)
type MessageHandler func(data string)

type Socket struct {
	Id         string
	connection net.Conn
	events     map[string]MessageHandler
	server     *Server
}

type Server struct {
	listener        net.Listener
	sockets         map[string]*Socket
	connectEvent    ConnectionHandler
	disconnectEvent ConnectionHandler
}

func (s *Server) addSocket(conn net.Conn) *Socket {
	uid := uuid.New().String()
	sock := &Socket{Id: uid, connection: conn, events: map[string]MessageHandler{}, server: s}
	s.sockets[uid] = sock
	return sock
}

func (s *Server) Start() {
	defer s.listener.Close()
	log.Println("Server started and listening for connections")

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Printf("Couldn't accept connection: %v\n", err)
		} else {
			go s.handleConnection(conn)
		}
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	// log.Printf("Accepted connection from %v\n", conn.RemoteAddr().String())
	socket := s.addSocket(conn)
	s.connectEvent(socket)

	sockBuffer := bufio.NewReader(conn)

	for {
		recv, err := sockBuffer.ReadString('\x00')
		if err != nil {
			// log.Printf("Socket %v disconnected\n", socket.uid)
			break
		}

		go func() {
			message := strings.TrimSpace(recv)
			if strings.Contains(message, DELIMITER) {
				parts := delimiterRegex.FindAllStringSubmatch(message, 2)
				if len(parts) == 1 && len(parts[0]) >= 3 {
					event := parts[0][1]

					if strings.Contains(event, "\\"+DELIMITER) {
						event = strings.ReplaceAll(event, "\\"+DELIMITER, DELIMITER)
					}

					if handler, ok := socket.events[event]; ok {
						data := parts[0][2]

						if strings.Contains(data, "\\"+DELIMITER) {
							data = strings.ReplaceAll(data, "\\"+DELIMITER, DELIMITER)
						}

						data = strings.Trim(data, "\x00\x01")

						go handler(data)
					}
				} else {
					log.Printf("Received a malformed message: %v\n", message)
				}
			}
		}()
	}
	conn.Close()
	if _, ok := s.sockets[socket.Id]; ok {
		delete(s.sockets, socket.Id)
	}
	s.disconnectEvent(socket)
}

func (s *Server) OnConnection(handler ConnectionHandler) {
	s.connectEvent = handler
}

func (s *Server) OnDisconnection(handler ConnectionHandler) {
	s.disconnectEvent = handler
}

func (s *Server) Emit(event, data string) {
	for _, socket := range s.sockets {
		go socket.Emit(event, data)
	}
	time.Sleep(time.Millisecond * 5)
}

func (s *Socket) On(event string, callback MessageHandler) {
	s.events[event] = callback
}

func (s *Socket) Off(event string) {
	if _, ok := s.events[event]; ok {
		delete(s.events, event)
	}
}

func (s *Socket) EmitSync(event, data string) {
	emit(s, event, data)
	time.Sleep(time.Millisecond * 3)
}

func (s *Socket) Emit(event, data string) {
	go emit(s, event, data)
}

func emit(socket *Socket, event, data string) {
	if strings.Contains(data, DELIMITER) {
		data = strings.ReplaceAll(data, DELIMITER, "\\"+DELIMITER)
	}
	if strings.ContainsRune(data, '\x00') {
		data = strings.ReplaceAll(data, "\x00", "\x01")
	}
	if strings.Contains(event, DELIMITER) {
		event = strings.ReplaceAll(event, DELIMITER, "\\"+DELIMITER)
	}
	if strings.ContainsRune(event, '\x00') {
		event = strings.ReplaceAll(event, "\x00", "\x01")
	}
	packet := []byte(fmt.Sprintf("%v%v%v", event, DELIMITER, data))
	packet = append(packet, '\x00')
	socket.connection.Write(packet)
}

func New(address string) (*Server, error) {
	l, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	return &Server{listener: l, sockets: map[string]*Socket{}, connectEvent: func(socket *Socket) {}, disconnectEvent: func(socket *Socket) {}}, nil
}
