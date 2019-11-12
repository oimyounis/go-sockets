package client

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

type FrameType byte

const (
	BATCH_SIZE               int       = 1024
	HEADER_SIZE              int       = 14
	FRAME_TYPE_MESSAGE       FrameType = 90
	FRAME_TYPE_HEARTBEAT     FrameType = 91
	FRAME_TYPE_HEARTBEAT_ACK FrameType = 92
)

// var (
// 	TERMINAL_SEQ     []byte = []byte{96, 96, 0, 96, 96}
// 	TERMINAL_SEQ_LEN int    = len(TERMINAL_SEQ)
// 	FILL_POINT       int    = BATCH_SIZE - len(TERMINAL_SEQ)
// )

type ConnectionHandler func(socket *Socket)
type MessageHandler func(data string)

type Socket struct {
	Id               string
	connection       net.Conn
	events           map[string]MessageHandler
	connected        bool
	lastHeartbeatAck int64
}

func (s *Socket) SendSync(event, data string) {
	send(s, event, data)
}

func (s *Socket) Send(event, data string) {
	go send(s, event, data)
}

func (s *Socket) Start() {
	s.envokeEvent("connection", "")
	go s.listen()
}

func (s *Socket) Listen() {
	s.envokeEvent("connection", "")
	// go s.startHeartbeat()
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

func (s *Socket) envokeEvent(name, data string) {
	if handler, ok := s.events[name]; ok {
		handler(data)
	}
}

func (s *Socket) startHeartbeat() {
	time.Sleep(time.Second * 5)
	for {
		if !s.connected {
			break
		}

		log.Println("sending heartbeat")
		start := time.Now().UnixNano() / 1000000
		raw(s, []byte{}, FRAME_TYPE_HEARTBEAT)
		time.Sleep(time.Second * 5)
		if !s.connected {
			break
		}
		log.Println("heartbeat wakeup", s.lastHeartbeatAck == 0, s.lastHeartbeatAck-start)
		if s.lastHeartbeatAck == 0 || s.lastHeartbeatAck-start > 5000 {
			log.Println("disconnecting from server")
			s.connected = false
			break
		}
	}
}

func (s *Socket) listen() {
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

		log.Printf("in > %v", recv)

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
							skip := 0
							for i := 0; i < dataLen; i++ {
								if skip > 1 {
									skip--
									continue
								}
								if data[i] == 92 && i != dataLen-2 && data[i+1] == 92 && data[i+2] == 0 {
									skip = 2
									continue
								}
								if skip == 1 {
									filtered = append(filtered, 10)
									skip--
								} else {
									filtered = append(filtered, data[i])
								}
							}

							go s.envokeEvent(eventName, string(filtered))
						}
					}
				case byte(FRAME_TYPE_HEARTBEAT):
					raw(s, []byte{}, FRAME_TYPE_HEARTBEAT_ACK)
				case byte(FRAME_TYPE_HEARTBEAT_ACK):
					s.lastHeartbeatAck = time.Now().UnixNano() / 1000000
				}
			}
		}(recv)
	}
	s.connection.Close()
	s.envokeEvent("disconnection", "")
}

func (s *Socket) Connected() bool {
	return s.connected
}

func (s *Socket) Disconnect() {
	s.connected = false
}

func (s *Socket) Emit(event string, data []byte) {
	emit(s, event, data)
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

	return frame, nil
}

var mu sync.Mutex

func randomBytes(size int) (buff []byte) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	mu.Lock()
	defer mu.Unlock()
	for i := 0; i < size; i++ {
		buff = append(buff, byte(r.Intn(255)))
	}
	return
}

func pad(buff []byte, size int) []byte {
	return append(buff, make([]byte, size-len(buff))...)
}

func emit(socket *Socket, event string, data []byte) {
	if !socket.connected {
		return
	}
	// frame, err := buildMessageFrame(event, data, frameType)
	// if err != nil {
	// 	return
	// }

	if len(event) > 1<<16-2 {
		return
		// return nil, fmt.Errorf("Event Name length exceeds the maximum of %v bytes\n", 1<<16-2)
	}

	dataLen := float64(len(data))
	srcLenBuff := make([]byte, 2)
	eventLenBuff := make([]byte, 2)
	eventBytes := []byte(event)
	eventLen := len(eventBytes)
	binary.BigEndian.PutUint16(eventLenBuff, uint16(eventLen))

	batchId := randomBytes(4)

	headerBuff := []byte{}
	headerBuff = append(headerBuff, byte(FRAME_TYPE_MESSAGE))
	headerBuff = append(headerBuff, batchId...)
	headerBuff = append(headerBuff, 0)

	headerBuff = append(headerBuff, eventLenBuff...)
	headerBuff = append(headerBuff, eventBytes...)
	headerBuff = append(headerBuff, 0, 0)

	headerBuffLen := len(headerBuff)

	batchCount := math.Ceil(dataLen / float64(BATCH_SIZE))

	allDataLen := float64(headerBuffLen)*batchCount + dataLen
	batchCount = math.Ceil(float64(allDataLen) / float64(BATCH_SIZE))

	realBatchSize := float64(BATCH_SIZE - headerBuffLen)

	lastEl := batchCount - 1

	count := 0

	now := time.Now()
	for b := 0.0; b < batchCount; b++ {
		frameBuff := []byte{}
		frameBuff = append(frameBuff, headerBuff...)

		start := int(b * realBatchSize)
		end := int(math.Min(dataLen-float64(start), realBatchSize))

		src := data[start : start+end]
		binary.BigEndian.PutUint16(srcLenBuff, uint16(len(src)))

		frameBuff[8+eventLen] = srcLenBuff[0]
		frameBuff[9+eventLen] = srcLenBuff[1]

		frameBuff = append(frameBuff, src...)

		if b == lastEl {
			frameBuff[5] = 1
			frameBuff = pad(frameBuff, BATCH_SIZE)
		} else {
			frameBuff[5] = 0
		}

		count += len(src)

		if _, err := socket.connection.Write(frameBuff); err != nil {
			return
		}
	}
	log.Println(time.Since(now), event, count)

	// log.Printf("out < %v\n", frame)
}

func buildFrame(data []byte, frameType FrameType) ([]byte, error) {
	frame := []byte{}
	frame = append(frame, byte(frameType))

	frame = append(frame, (bytes.ReplaceAll(data, []byte{10}, []byte{92, 92, 0}))...)
	frame = append(frame, 10)

	return frame, nil
}

func raw(socket *Socket, data []byte, frameType FrameType) {
	if !socket.connected {
		return
	}
	frame, err := buildFrame(data, frameType)
	if err != nil {
		return
	}
	log.Printf("out < %v\n", frame)
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
