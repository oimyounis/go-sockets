package client

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

type FrameType byte

const (
	FRAME_TYPE_MESSAGE       FrameType = 90
	FRAME_TYPE_HEARTBEAT     FrameType = 91
	FRAME_TYPE_HEARTBEAT_ACK FrameType = 92
)

type ConnectionHandler func(socket *Socket)
type MessageHandler func(data string)

type Socket struct {
	connection       net.Conn
	events           map[string]MessageHandler
	connectEvent     ConnectionHandler
	disconnectEvent  ConnectionHandler
	connected        bool
	lastHeartbeatAck int64
}

func (s *Socket) EmitSync(event, data string) {
	emit(s, event, data)
	time.Sleep(time.Millisecond * 2)
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

func (s *Socket) Off(event string) {
	if _, ok := s.events[event]; ok {
		delete(s.events, event)
	}
}

func (s *Socket) Connection() net.Conn {
	return s.connection
}
func (s *Socket) socketReceiver() {
	sockBuffer := bufio.NewReader(s.connection)
	for {
		if !s.connected {
			break
		}

		recv, err := sockBuffer.ReadBytes(10)
		if err != nil {
			// log.Println(err)
			break
		}

		go func(frame []byte) {
			if !s.connected {
				return
			}
			frameLen := len(frame)

			if frameLen > 0 {
				switch frame[0] {
				case byte(FRAME_TYPE_MESSAGE):
					if frameLen > 2 {
						eventLen := binary.BigEndian.Uint16(frame[1:3])
						eventName := strings.Trim(string(frame[3:3+eventLen]), "\x00")
						if frameLen > 3+int(eventLen) {
							data := frame[3+eventLen : frameLen-1]
							dataLen := len(data)

							filtered := make([]byte, 0, dataLen)
							convert := false
							for i := 0; i < dataLen; i++ {
								if data[i] == 92 && i != dataLen-1 && data[i+1] == 0 {
									convert = true
									continue
								}
								if !convert {
									filtered = append(filtered, data[i])
								} else {
									filtered = append(filtered, 10)
									convert = false
								}
							}

							if handler, ok := s.events[eventName]; ok {
								go handler(string(filtered))
							}
						}
					}
				case byte(FRAME_TYPE_HEARTBEAT):
					raw(s, []byte{}, FRAME_TYPE_HEARTBEAT_ACK)
				case byte(FRAME_TYPE_HEARTBEAT_ACK):
					s.lastHeartbeatAck = time.Now().Unix()
				}
			}
		}(recv)
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

func (s *Socket) Disconnect() {
	s.connected = false
}

func (s *Socket) Send(event string, data []byte) {
	send(s, event, data, FRAME_TYPE_MESSAGE)
}

func buildMessageFrameHeader(event string, frameType FrameType) ([]byte, error) {
	if len(event) > 1<<16-2 {
		return nil, fmt.Errorf("Event Name length exceeds the maximum of %v bytes\n", 1<<16-2)
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

func buildMessageFrame(event string, data []byte, frameType FrameType) ([]byte, error) {
	frame, err := buildMessageFrameHeader(event, frameType)
	if err != nil {
		return nil, err
	}

	frame = append(frame, (bytes.ReplaceAll(data, []byte{10}, []byte{92, 0}))...)
	frame = append(frame, 10)

	return frame, nil
}

func buildFrame(data []byte, frameType FrameType) ([]byte, error) {
	frame := []byte{}
	frame = append(frame, byte(frameType))

	frame = append(frame, (bytes.ReplaceAll(data, []byte{10}, []byte{92, 0}))...)
	frame = append(frame, 10)

	return frame, nil
}

func send(socket *Socket, event string, data []byte, frameType FrameType) {
	frame, err := buildMessageFrame(event, data, frameType)
	if err != nil {
		return
	}
	log.Printf("out > %v\n", frame)
	if _, err = socket.connection.Write(frame); err != nil {
		return
	}
}

func raw(socket *Socket, data []byte, frameType FrameType) {
	frame, err := buildFrame(data, frameType)
	if err != nil {
		return
	}
	log.Printf("out > %v\n", frame)
	if _, err = socket.connection.Write(frame); err != nil {
		return
	}
}

func emit(socket *Socket, event, data string) {
	send(socket, event, []byte(data), FRAME_TYPE_MESSAGE)
}

func New(address string) (*Socket, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	return &Socket{
		connection:      conn,
		events:          map[string]MessageHandler{},
		connectEvent:    func(socket *Socket) {},
		disconnectEvent: func(socket *Socket) {},
		connected:       true,
	}, nil
}
