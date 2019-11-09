package server

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
)

const DELIMITER = "§"
const DELIMITER_LENGTH = len(DELIMITER)

type ConnectionHandler func(socket *Socket)
type MessageHandler func(socket *Socket, data string)

type Socket struct {
	Id         string
	connection net.Conn
	events     map[string]MessageHandler
}

type Server struct {
	listener        net.Listener
	sockets         map[string]*Socket
	connectEvent    ConnectionHandler
	disconnectEvent ConnectionHandler
}

func (s *Server) addSocket(conn net.Conn) *Socket {
	uid := uuid.New().String()
	sock := &Socket{Id: uid, connection: conn, events: map[string]MessageHandler{}}
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

	for {
		recv, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			// log.Printf("Socket %v disconnected\n", socket.uid)
			if _, ok := s.sockets[socket.Id]; ok {
				delete(s.sockets, socket.Id)
			}
			break
		}

		go func() {
			message := strings.TrimSpace(recv)
			if strings.Contains(message, DELIMITER) {
				delimIdx := strings.Index(message, DELIMITER)
				event := message[:delimIdx]
				if handler, ok := socket.events[event]; ok {
					data := message[delimIdx+DELIMITER_LENGTH:]
					if strings.Contains(data, "\n") {
						data = strings.ReplaceAll(data, "\\n", "\n")
					}
					go handler(socket, data)
				}
			}
		}()
	}
	conn.Close()
	s.disconnectEvent(socket)
}

func (s *Server) OnConnect(handler ConnectionHandler) {
	s.connectEvent = handler
}

func (s *Server) OnDisconnect(handler ConnectionHandler) {
	s.disconnectEvent = handler
}

func (s *Socket) On(event string, callback MessageHandler) {
	s.events[event] = callback
}

func (s *Socket) EmitSync(event, data string) {
	emit(s, event, data)
	time.Sleep(time.Millisecond * 5)
}

func (s *Socket) Emit(event, data string) {
	go emit(s, event, data)
}

func emit(socket *Socket, event, data string) {
	// time.Sleep(time.Millisecond * 10)
	if strings.Contains(data, "\n") {
		data = strings.ReplaceAll(data, "\n", "\\n")
	}
	socket.connection.Write([]byte(fmt.Sprintf("%v%v%v\n", event, DELIMITER, data)))
}

func New(address string) *Server {
	l, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Failed to start server: %v\n", err)
	}
	s := &Server{listener: l, sockets: map[string]*Socket{}, connectEvent: func(socket *Socket) {}, disconnectEvent: func(socket *Socket) {}}
	return s
}