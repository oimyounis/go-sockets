package client

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"
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
	connected        bool
	lastHeartbeatAck int64
}

func (s *Socket) Start() {
	s.envokeEvent("connection", "")
	go s.listen()
}

func (s *Socket) Listen() {
	s.envokeEvent("connection", "")
	go s.startHeartbeat()
	s.listen()
}

func (s *Socket) On(event string, callback MessageHandler) {
	s.events[event] = callback
}

func (s *Socket) Off(event string) {
	if _, ok := s.events[event]; ok {
		delete(s.events, event)
	}
}

func (s *Socket) Connection() net.Conn {
	return s.connection
}

func (s *Socket) disconnect() {
	if !s.connected {
		s.connection.Close()
		return
	}
	s.connected = false
	s.connection.Close()
	s.envokeEvent("disconnection", "")
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

func (s *Socket) Connected() bool {
	return s.connected
}

func (s *Socket) Disconnect() {
	s.disconnect()
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

func New(address string) (*Socket, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	return &Socket{
		connection: conn,
		events:     map[string]MessageHandler{},
		connected:  true,
	}, nil
}
