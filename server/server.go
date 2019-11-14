package server

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/google/uuid"
)

type FrameType byte

const (
	FRAME_SIZE               int       = 1024
	FRAME_TYPE_MESSAGE       FrameType = 90
	FRAME_TYPE_HEARTBEAT     FrameType = 91
	FRAME_TYPE_HEARTBEAT_ACK FrameType = 92
)

const (
	HEARTBEAT_INTERVAL = 10
)

type ConnectionHandler func(socket *Socket)
type MessageHandler func(data string)

type Socket struct {
	Id               string
	connection       net.Conn
	events           map[string]MessageHandler
	server           *Server
	connected        bool
	lastHeartbeatAck int64
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

func (s *Server) Connection() net.Listener {
	return s.listener
}

func (s *Socket) On(event string, callback MessageHandler) {
	s.events[event] = callback
}

func (s *Socket) Off(event string) {
	if _, ok := s.events[event]; ok {
		delete(s.events, event)
	}
}

// Under development. Does not guarantee 100% synchronization
func (s *Socket) SendSync(event, data string) {
	send(s, event, data)
}

func (s *Socket) Send(event, data string) {
	go send(s, event, data)
}

func (s *Socket) Emit(event string, data []byte) {
	go emit(s, event, data)
}

// Under development. Does not guarantee 100% synchronization
func (s *Socket) EmitSync(event string, data []byte) {
	emit(s, event, data)
}

// func (s *Socket) BroadcastSync(event, data string) {
// 	// for id, socket := range s.server.sockets {
// 	// 	if id == s.Id {
// 	// 		continue
// 	// 	}
// 	// 	go socket.Emit(event, data)
// 	// }
// 	time.Sleep(time.Millisecond * 2)
// }

func (s *Socket) Broadcast(event, data string) {
	for id, socket := range s.server.sockets {
		if id == s.Id {
			continue
		}
		go socket.Send(event, data)
	}
}

func (s *Socket) Connected() bool {
	return s.connected
}

func (s *Socket) Connection() net.Conn {
	return s.connection
}

func (s *Socket) Disconnect() {
	s.disconnect()
}

func (s *Socket) disconnect() {
	if !s.connected {
		s.connection.Close()
		return
	}
	s.connected = false
	s.connection.Close()
	s.server.disconnectEvent(s)
}

func (s *Socket) envokeEvent(name, data string) {
	if handler, ok := s.events[name]; ok {
		handler(data)
	}
}

func (s *Socket) startHeartbeat() {
	time.Sleep(time.Second * 2)
	for {
		if !s.connected {
			break
		}

		// log.Println("sending heartbeat")
		start := time.Now().UnixNano() / 1000000
		raw(s, []byte{}, FRAME_TYPE_HEARTBEAT)
		time.Sleep(time.Second * HEARTBEAT_INTERVAL)
		if !s.connected {
			break
		}
		if s.lastHeartbeatAck == 0 || s.lastHeartbeatAck-start > HEARTBEAT_INTERVAL*1000 {
			// log.Println("disconnecting from server")
			break
		}
		log.Println("HEARTBEAT OK")
	}
	s.disconnect()
}

func (s *Server) handleConnection(conn net.Conn) {
	// log.Printf("Accepted connection from %v\n", conn.RemoteAddr().String())
	socket := s.addSocket(conn)
	s.connectEvent(socket)
	go socket.startHeartbeat()
	socket.listen()
}

func (s *Socket) listen() {
	sockBuffer := bufio.NewReader(s.connection)

	for {
		if !s.connected {
			break
		}

		size := make([]byte, 4)
		n, err := sockBuffer.Read(size)
		if err != nil || n != 4 {
			// log.Println("err", err, n)
			break
		}

		sizeVal := int(binary.BigEndian.Uint32(size))

		payload := make([]byte, sizeVal)
		n, err = io.ReadFull(sockBuffer, payload)
		if err != nil || n != sizeVal {
			// log.Println("err2", err, n, len(payload))
			break
		}

		frameType := payload[0]

		switch frameType {
		case byte(FRAME_TYPE_MESSAGE):
			processMessageFrame(s, payload[1:])
		case byte(FRAME_TYPE_HEARTBEAT):
			raw(s, []byte{}, FRAME_TYPE_HEARTBEAT_ACK)
		case byte(FRAME_TYPE_HEARTBEAT_ACK):
			s.lastHeartbeatAck = time.Now().UnixNano() / 1000000
		default:
			log.Fatalln("unknown frame type", frameType, payload)
		}
	}
	s.disconnect()
}

func processMessageFrame(s *Socket, frame []byte) {
	frameLen := len(frame)
	if frameLen > 3 {
		eventLen := binary.BigEndian.Uint16(frame[:2])
		eventEnd := int(2 + eventLen)
		eventName := string(frame[2:eventEnd])

		data := frame[eventEnd:]

		go s.envokeEvent(eventName, string(data))
	}
}

func emit(socket *Socket, event string, data []byte) error {
	if !socket.connected {
		return errors.New("Connection is already closed")
	}

	if len(event) > 1<<16-2 {
		return fmt.Errorf("Event Name length exceeds the maximum of %v bytes\n", 1<<16-2)
	}

	headerBuff := []byte{0, 0, 0, 0}

	payload := []byte{0, 0}

	eventBytes := []byte(event)
	eventLen := len(eventBytes)
	binary.BigEndian.PutUint16(payload, uint16(eventLen))

	payload = append(payload, eventBytes...)
	payload = append(payload, data...)

	binary.BigEndian.PutUint32(headerBuff, uint32(len(payload)+1))
	headerBuff = append(headerBuff, byte(FRAME_TYPE_MESSAGE))

	frameBuff := []byte{}
	frameBuff = append(frameBuff, headerBuff...)
	frameBuff = append(frameBuff, payload...)

	time.Sleep(time.Microsecond * 500)
	if _, err := socket.connection.Write(frameBuff); err != nil {
		return fmt.Errorf("Error writing to underlying connection: %v", err)
	}
	return nil
}

func buildFrame(data []byte, frameType FrameType) ([]byte, error) {
	headerBuff := []byte{0, 0, 0, 0}

	payload := []byte{}

	payload = append(payload, data...)

	binary.BigEndian.PutUint32(headerBuff, uint32(len(payload)+1))
	headerBuff = append(headerBuff, byte(frameType))

	frameBuff := []byte{}
	frameBuff = append(frameBuff, headerBuff...)
	frameBuff = append(frameBuff, payload...)

	return frameBuff, nil
}

func raw(socket *Socket, data []byte, frameType FrameType) {
	if !socket.connected {
		return
	}
	frame, err := buildFrame(data, frameType)
	if err != nil {
		return
	}
	time.Sleep(time.Microsecond * 500)

	if _, err = socket.connection.Write(frame); err != nil {
		return
	}
}

func send(socket *Socket, event, data string) {
	emit(socket, event, []byte(data))
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
