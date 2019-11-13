package client

import (
	"bufio"
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
	RAW_HEADER_SIZE          int       = 5
	FRAME_SIZE               int       = 1024
	FRAME_TYPE_MESSAGE       FrameType = 90
	FRAME_TYPE_HEARTBEAT     FrameType = 91
	FRAME_TYPE_HEARTBEAT_ACK FrameType = 92
)

const (
	HEARTBEAT_INTERVAL = 1
)

// var (
// 	TERMINAL_SEQ     []byte = []byte{96, 96, 0, 96, 96}
// 	TERMINAL_SEQ_LEN int    = len(TERMINAL_SEQ)
// 	FILL_POINT       int    = FRAME_SIZE - len(TERMINAL_SEQ)
// )

type ConnectionHandler func(socket *Socket)
type MessageHandler func(data string)

type Socket struct {
	Id               string
	connection       net.Conn
	events           map[string]MessageHandler
	connected        bool
	lastHeartbeatAck int64
	TotalSentBytes   uint64
	// mutex            sync.Mutex
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
	time.Sleep(time.Second * 5)
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
			// s.disconnect()
			break
		}
		log.Println("HEARTBEAT OK")
	}
	s.disconnect()
}

func (s *Socket) listen() {
	sockBuffer := bufio.NewReader(s.connection)
	batchQueue := make(map[uint32][]byte)

	for {
		if !s.connected {
			break
		}

		frame := make([]byte, FRAME_SIZE)
		n, err := sockBuffer.Read(frame)
		if err != nil {
			// log.Println(err)
			break
		}

		// log.Printf("in [%v] > %v", n, frame[:10])

		// go func(frame []byte) {
		if !s.connected {
			return
		}
		frameLen := len(frame)

		if n == FRAME_SIZE {
			switch frame[0] {
			case byte(FRAME_TYPE_MESSAGE):
				if frameLen > 13 {
					batchId := binary.BigEndian.Uint32(frame[1:5])

					isLast := frame[5:6][0] == 1
					// log.Println("isLast", isLast)

					eventLen := binary.BigEndian.Uint16(frame[6:8])
					eventEnd := 8 + eventLen
					if frameLen < int(eventEnd) {
						log.Println("frame", n, frame)
						log.Println("eventEnd", eventEnd)
						log.Println("frame[8]", frame[8])
						continue
					}

					eventName := string(frame[8:eventEnd])
					// log.Println("eventLen", eventLen, "eventName", eventName)

					dataLen := binary.BigEndian.Uint16(frame[eventEnd : eventEnd+2])

					data := frame[eventEnd+2 : eventEnd+2+dataLen]

					batchQueue[batchId] = append(batchQueue[batchId], data...)

					if isLast {
						go s.envokeEvent(eventName, string(batchQueue[batchId]))
						delete(batchQueue, batchId)
					}
				}
			case byte(FRAME_TYPE_HEARTBEAT):
				raw(s, []byte{}, FRAME_TYPE_HEARTBEAT_ACK)
			case byte(FRAME_TYPE_HEARTBEAT_ACK):
				s.lastHeartbeatAck = time.Now().UnixNano() / 1000000
			}
		} else if n >= RAW_HEADER_SIZE {
			processedBytes := 0
			for processedBytes != n {
				frameType := frame[processedBytes : processedBytes+1][0]
				dataLen := int(binary.BigEndian.Uint32(frame[processedBytes+1 : processedBytes+RAW_HEADER_SIZE]))
				data := []byte{}
				if dataLen > 0 {
					data = frame[processedBytes+RAW_HEADER_SIZE : processedBytes+RAW_HEADER_SIZE+dataLen]
				}
				processRawFrame(s, frameType, data)
				processedBytes += dataLen + RAW_HEADER_SIZE
			}
		}
		// }(buff)
	}
	s.disconnect()
}

func processRawFrame(s *Socket, frameType byte, data []byte) {
	switch frameType {
	case byte(FRAME_TYPE_HEARTBEAT):
		raw(s, []byte{}, FRAME_TYPE_HEARTBEAT_ACK)
	case byte(FRAME_TYPE_HEARTBEAT_ACK):
		s.lastHeartbeatAck = time.Now().UnixNano() / 1000000
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

	dataLen := len(data)
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

	realBatchSize := FRAME_SIZE - headerBuffLen
	batchCount := int(math.Ceil(float64(dataLen) / float64(realBatchSize)))

	allDataLen := headerBuffLen*batchCount + dataLen
	batchCount = int(math.Ceil(float64(allDataLen) / float64(FRAME_SIZE)))

	lastEl := batchCount - 1

	frameBuff := []byte{}
	// count := 0
	// now := time.Now()

	for b := 0; b < batchCount; b++ {
		frameBuff = append(frameBuff, headerBuff...)

		start := b * realBatchSize
		end := int(math.Min(float64(dataLen-start), float64(realBatchSize)))

		src := data[start : start+end]

		// count++

		binary.BigEndian.PutUint16(srcLenBuff, uint16(len(src)))

		chunkStart := b * FRAME_SIZE

		frameBuff[chunkStart+8+eventLen] = srcLenBuff[0]
		frameBuff[chunkStart+9+eventLen] = srcLenBuff[1]

		frameBuff = append(frameBuff, src...)

		if b == lastEl {
			frameBuff[chunkStart+5] = 1
		} else {
			frameBuff[chunkStart+5] = 0
		}
	}

	frameBuff = pad(frameBuff, batchCount*FRAME_SIZE)

	// socket.mutex.Lock()
	socket.TotalSentBytes += uint64(len(frameBuff))
	if _, err := socket.connection.Write(frameBuff); err != nil {
		// socket.mutex.Unlock()
		return
	}
	// socket.mutex.Unlock()

	// log.Println(time.Since(now), event, len(frameBuff))

	// log.Printf("out < %v\n", frame)
}

func buildFrame(data []byte, frameType FrameType) ([]byte, error) {
	frame := []byte{}
	frame = append(frame, byte(frameType))

	dataLenBuff := make([]byte, 4)
	binary.BigEndian.PutUint32(dataLenBuff, uint32(len(data)))

	frame = append(frame, dataLenBuff...)
	frame = append(frame, data...)
	// frame = pad(frame, FRAME_SIZE)

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
	// log.Printf("out < %v\n", frameType)
	socket.TotalSentBytes += uint64(len(frame))
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
