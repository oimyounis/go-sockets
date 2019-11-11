package client

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"
)

type FrameType byte

const (
	DELIMITER                      = "§"
	DELIMITER_LENGTH               = len(DELIMITER)
	FRAME_TYPE_MESSAGE   FrameType = 1
	FRAME_TYPE_HEARTBEAT FrameType = 2
)

// var delimiterRegex *regexp.Regexp = regexp.MustCompile(`(.*[^\\])§((.*\n?.*)*)`)

type ConnectionHandler func(socket *Socket)
type MessageHandler func(data string)

type Socket struct {
	connection      net.Conn
	events          map[string]MessageHandler
	connectEvent    ConnectionHandler
	disconnectEvent ConnectionHandler
	connected       bool
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
				if frame[0] == byte(FRAME_TYPE_MESSAGE) {
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
				}
			}
		}(recv)

		// go func() {
		// 	if !s.connected {
		// 		return
		// 	}

		// 	message := strings.TrimSpace(recv)
		// 	if strings.Contains(message, DELIMITER) {
		// 		parts := delimiterRegex.FindAllStringSubmatch(message, 2)
		// 		if len(parts) == 1 && len(parts[0]) >= 3 {
		// 			event := parts[0][1]

		// 			if strings.Contains(event, "\\"+DELIMITER) {
		// 				event = strings.ReplaceAll(event, "\\"+DELIMITER, DELIMITER)
		// 			}

		// 			if handler, ok := s.events[event]; ok {
		// 				data := parts[0][2]

		// 				if strings.Contains(data, "\\"+DELIMITER) {
		// 					data = strings.ReplaceAll(data, "\\"+DELIMITER, DELIMITER)
		// 				}

		// 				data = strings.Trim(data, "\x00\x01")

		// 				go handler(data)
		// 			}
		// 		} else {
		// 			log.Printf("Received a malformed message: %v\n", message)
		// 		}
		// 	}
		// }()
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
