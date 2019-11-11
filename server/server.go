package server

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type FrameType byte

const (
	DELIMITER                      = "§"
	DELIMITER_LENGTH               = len(DELIMITER)
	FRAME_TYPE_MESSAGE   FrameType = 1
	FRAME_TYPE_HEARTBEAT FrameType = 2
)

var delimiterRegex *regexp.Regexp = regexp.MustCompile(`(.*[^\\])§((.*\n?.*)*)`)

type ConnectionHandler func(socket *Socket)
type MessageHandler func(data string)

type Socket struct {
	Id         string
	connection net.Conn
	events     map[string]MessageHandler
	server     *Server
	connected  bool
}

type Server struct {
	address         string
	listener        net.Listener
	sockets         map[string]*Socket
	connectEvent    ConnectionHandler
	disconnectEvent ConnectionHandler
}

func (s *Server) addSocket(conn net.Conn) *Socket {
	uid := uuid.New().String()
	sock := &Socket{Id: uid, connection: conn, events: map[string]MessageHandler{}, server: s, connected: true}
	s.sockets[uid] = sock
	return sock
}

func (s *Server) removeSocket(socket *Socket) {
	if _, ok := s.sockets[socket.Id]; ok {
		socket.connected = false
		delete(s.sockets, socket.Id)
	}
}

func (s *Server) Listen() {
	defer s.listener.Close()
	log.Println("Server listening on " + s.listener.Addr().String())

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Printf("Couldn't accept connection: %v\n", err)
		} else {
			go s.handleConnection(conn)
		}
	}
}

func (s *Server) OnConnection(handler ConnectionHandler) {
	s.connectEvent = handler
}

func (s *Server) OnDisconnection(handler ConnectionHandler) {
	s.disconnectEvent = handler
}

func (s *Server) EmitSync(event, data string) {
	for _, socket := range s.sockets {
		go socket.Emit(event, data)
	}
	time.Sleep(time.Millisecond * 2)
}

func (s *Server) Emit(event, data string) {
	for _, socket := range s.sockets {
		go socket.Emit(event, data)
	}
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
	time.Sleep(time.Millisecond * 2)
}

func (s *Socket) Emit(event, data string) {
	go emit(s, event, data)
}

func (s *Socket) BroadcastSync(event, data string) {
	for id, socket := range s.server.sockets {
		if id == s.Id {
			continue
		}
		go socket.Emit(event, data)
	}
	time.Sleep(time.Millisecond * 2)
}

func (s *Socket) Broadcast(event, data string) {
	for id, socket := range s.server.sockets {
		if id == s.Id {
			continue
		}
		go socket.Emit(event, data)
	}
}

func (s *Socket) Connected() bool {
	return s.connected
}

func (s *Socket) Disconnect() {
	s.connected = false
}

func (s *Socket) Send(event string, data []byte) {
	send(s, event, data, FRAME_TYPE_MESSAGE)
}

func (s *Server) handleConnection(conn net.Conn) {
	// log.Printf("Accepted connection from %v\n", conn.RemoteAddr().String())
	socket := s.addSocket(conn)
	s.connectEvent(socket)
	socket.listen()
}

func (s *Socket) listen() {
	sockBuffer := bufio.NewReader(s.connection)

	for {
		if !s.connected {
			break
		}

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

					if handler, ok := s.events[event]; ok {
						data := parts[0][2]

						if strings.Contains(data, "\\"+DELIMITER) {
							data = strings.ReplaceAll(data, "\\"+DELIMITER, DELIMITER)
						}

						data = strings.Trim(data, "\x00\x01")

						if !s.connected {
							return
						}

						go handler(data)
					}
				} else {
					log.Printf("Received a malformed message: %v\n", message)
				}
			}
		}()
	}
	s.connection.Close()
	s.server.removeSocket(s)
	s.server.disconnectEvent(s)
}

func buildFrameHeader(event string, frameType FrameType) ([]byte, error) {
	if len(event) > 1<<16-1 {
		return nil, fmt.Errorf("Event Name length exceeds the maximum of %v bytes\n", 1<<16-1)
	}

	frameBuff := []byte{}
	frameBuff = append(frameBuff, byte(frameType))

	event = strings.ReplaceAll(event, "\n", "")

	eventLenBuff := make([]byte, 2)
	eventBytes := []byte(event)
	eventLen := len(eventBytes)

	if eventLen/256 == 10 {
		for i := 0; i < 256-eventLen%256; i++ {
			eventBytes = append(eventBytes, 0)
		}
	} else if eventLen%256 == 10 {
		eventBytes = append(eventBytes, 0)
	}

	binary.BigEndian.PutUint16(eventLenBuff, uint16(len(eventBytes)))
	frameBuff = append(frameBuff, eventLenBuff...)
	frameBuff = append(frameBuff, eventBytes...)

	return frameBuff, nil
}

func buildFrame(event string, data []byte, frameType FrameType) ([]byte, error) {
	frame, err := buildFrameHeader(event, frameType)
	if err != nil {
		return nil, err
	}

	frame = append(frame, (bytes.ReplaceAll(data, []byte{10}, []byte{92, 0}))...)
	frame = append(frame, 10)

	return frame, nil
}

func send(socket *Socket, event string, data []byte, frameType FrameType) {
	frame, err := buildFrame(event, data, frameType)
	if err != nil {
		return
	}
	// log.Printf("out > %v\n", frame)
	if _, err = socket.connection.Write(frame); err != nil {
		return
	}
}

func emit(socket *Socket, event, data string) {
	send(socket, event, []byte(data), FRAME_TYPE_MESSAGE)
	// if strings.Contains(data, DELIMITER) {
	// 	data = strings.ReplaceAll(data, DELIMITER, "\\"+DELIMITER)
	// }
	// if strings.ContainsRune(data, '\x00') {
	// 	data = strings.ReplaceAll(data, "\x00", "\x01")
	// }
	// if strings.Contains(event, DELIMITER) {
	// 	event = strings.ReplaceAll(event, DELIMITER, "\\"+DELIMITER)
	// }
	// if strings.ContainsRune(event, '\x00') {
	// 	event = strings.ReplaceAll(event, "\x00", "\x01")
	// }
	// packet := []byte(fmt.Sprintf("%v%v%v", event, DELIMITER, data))
	// packet = append(packet, '\x00')
	// socket.connection.Write(packet)
}

func New(address string) (*Server, error) {
	l, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	return &Server{
		address:         address,
		listener:        l,
		sockets:         map[string]*Socket{},
		connectEvent:    func(socket *Socket) {},
		disconnectEvent: func(socket *Socket) {},
	}, nil
}
