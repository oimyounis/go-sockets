package client

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type FrameType byte

const (
	FRAME_SIZE               int       = 4096
	FRAME_TYPE_MESSAGE       FrameType = 90
	FRAME_TYPE_HEARTBEAT     FrameType = 91
	FRAME_TYPE_HEARTBEAT_ACK FrameType = 92
	FRAME_TYPE_READY         FrameType = 93
)

const (
	HEARTBEAT_INTERVAL = 2
)

type ConnectionHandler func(socket *Socket)
type MessageHandler func(data string)

type BuffQueue struct {
	currentIndex int
	queue        [][]byte
	mutex        sync.RWMutex
}

func (q *BuffQueue) len() int {
	return len(q.queue)
}

func (q *BuffQueue) next() []byte {
	if q.currentIndex == q.len() {
		return nil
	}
	q.mutex.RLock()
	defer q.mutex.RUnlock()
	buff := q.queue[q.currentIndex]
	q.currentIndex++
	return buff
}

func (q *BuffQueue) append(buff []byte) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.queue = append(q.queue, buff)
}

type RoundRobinBuffer struct {
	queue sync.Map
	mutex sync.RWMutex
}

func (rrb *RoundRobinBuffer) append(seq int, buff []byte) {
	rrb.mutex.Lock()
	defer rrb.mutex.Unlock()

	q, _ := rrb.queue.LoadOrStore(seq, &BuffQueue{})

	if q != nil {
		if q, ok := q.(*BuffQueue); ok {
			q.append(buff)
		}
	}
}

func (rrb *RoundRobinBuffer) clean(seq int) {
	rrb.mutex.Lock()
	defer rrb.mutex.Unlock()
	// delete(rrb.queue, seq)
	rrb.queue.Store(seq, nil)
}

func NewRoundRobinBuffer() *RoundRobinBuffer {
	return &RoundRobinBuffer{}
}

type Sequencer struct {
	current        int64
	UpperBoundBits uint
	mutex          sync.Mutex
}

func (seq *Sequencer) Next() int64 {
	seq.mutex.Lock()
	defer seq.mutex.Unlock()
	if seq.current == 1<<seq.UpperBoundBits-1 {
		seq.current = 0
	}

	next := seq.current
	seq.current++
	return next
}

func (seq *Sequencer) Val() int64 {
	if seq.current == 0 {
		return seq.current
	}
	return seq.current - 1
}

type Socket struct {
	Id               string
	address          string
	connection       net.Conn
	events           map[string]MessageHandler
	connected        bool
	lastHeartbeatAck int64
	sequence         *Sequencer
	buffer           *RoundRobinBuffer
	bytesSent        uint64
}

func (s *Socket) Start() error {
	err := s.connect()
	if err != nil {
		return err
	}
	s.envokeEvent("connection", "")
	go s.listen()
	return nil
}

func (s *Socket) Listen() error {
	err := s.connect()
	if err != nil {
		return err
	}
	s.envokeEvent("connection", "")
	go s.processSendQueue()
	go s.startHeartbeat()
	s.listen()
	return nil
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

func (s *Socket) connect() error {
	conn, err := net.Dial("tcp", s.address)
	if err != nil {
		return err
	}
	s.connection = conn
	s.buffer = NewRoundRobinBuffer()
	return nil
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

func (s *Socket) processSendQueue() {
	go func() {
		for {
			time.Sleep(time.Second * 5)
			log.Println(s.bytesSent)
		}
	}()
	// var printLock sync.Mutex
	for {
		if !s.connected {
			break
		}

		if s.bytesSent >= 512000 {
			time.Sleep(time.Millisecond)
			continue
		}

		s.buffer.queue.Range(func(i, queue interface{}) bool {
			if queue == nil {
				return true
			}
			if queue, ok := queue.(*BuffQueue); ok {
				if frame := queue.next(); frame != nil {
					rawBytes(s, frame)
				} else {
					if i, iok := i.(int); iok {
						s.buffer.clean(i)
					}
					return true
				}
			}

			time.Sleep(time.Millisecond * 1)

			return true
		})

		// printLock.Lock()
		// log.Println(s.buffer.queue)
		// printLock.Unlock()

		// time.Sleep(time.Second * 1)

		// ks := s.buffer.keys()
		// log.Println("keys", ks)
		// for key := range ks {
		// 	if queue, ok := s.buffer.queue[key]; ok {
		// 		log.Println("queue ok", ok)
		// 		if frame := queue.next(); frame != nil {
		// 			log.Println("frame head", frame[:6])
		// 			rawBytes(s, frame)
		// 		} else {
		// 			s.buffer.clean(key)
		// 		}
		// 	}

		// 	// time.Sleep(time.Second * 1)
		// 	// ks = s.buffer.keys()
		// }

	}
}

func (s *Socket) startHeartbeat() {
	// time.Sleep(time.Second * 2)
	for {
		if !s.connected {
			break
		}

		start := time.Now().UnixNano() / 1000000
		// log.Println("sending heartbeat", start)
		raw(s, []byte{}, FRAME_TYPE_HEARTBEAT)
		time.Sleep(time.Second * HEARTBEAT_INTERVAL)
		if !s.connected {
			break
		}
		if s.lastHeartbeatAck == 0 || s.lastHeartbeatAck-start > HEARTBEAT_INTERVAL*1000 {
			// log.Println("disconnecting from server")
			// break
		}
		// log.Println("HEARTBEAT OK")
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

	bufferedBytes := 0
	eventBytes := []byte(event)
	eventLen := len(eventBytes)
	payload := []byte{0, 0}

	binary.BigEndian.PutUint16(payload, uint16(eventLen))
	payload = append(payload, eventBytes...)
	payload = append(payload, data...)

	dataLen := len(payload)

	log.Println("emit sending", dataLen, "bytes")

	header := []byte{0, 0, 0, 0, 0, byte(FRAME_TYPE_MESSAGE)}

	seq := uint16(socket.sequence.Next())
	binary.BigEndian.PutUint16(header[2:4], seq)

	headerLen := len(header)
	usable := FRAME_SIZE - headerLen

	for bufferedBytes < dataLen {
		frameEnd := bufferedBytes + (usable)
		if frameEnd > dataLen {
			frameEnd = bufferedBytes + (dataLen - bufferedBytes)
		}
		dataSlice := payload[bufferedBytes:frameEnd]
		sliceLen := len(dataSlice)

		binary.BigEndian.PutUint16(header, uint16(sliceLen))

		buff := []byte{}

		buff = append(buff, header...)
		buff = append(buff, dataSlice...)

		bufferedBytes += sliceLen

		if bufferedBytes == dataLen {
			buff[4] = 2
		} else if bufferedBytes != 0 {
			header[4] = 1
		}

		socket.buffer.append(int(seq), buff)
		time.Sleep(time.Microsecond * 500)
	}

	// log.Println("finished sending", dataLen, "bytes")

	// headerBuff := []byte{0, 0, 0, 0}

	// payload := []byte{0, 0}

	// eventBytes := []byte(event)
	// eventLen := len(eventBytes)
	// binary.BigEndian.PutUint16(payload, uint16(eventLen))

	// payload = append(payload, eventBytes...)
	// payload = append(payload, data...)

	// binary.BigEndian.PutUint32(headerBuff, uint32(len(payload)+1))
	// headerBuff = append(headerBuff, byte(FRAME_TYPE_MESSAGE))

	// frameBuff := []byte{}
	// frameBuff = append(frameBuff, headerBuff...)
	// frameBuff = append(frameBuff, payload...)

	// time.Sleep(time.Microsecond * 500)
	// fmt.Println("writing", len(frameBuff), "bytes")
	// if _, err := socket.connection.Write(frameBuff); err != nil {
	// 	return fmt.Errorf("Error writing to underlying connection: %v", err)
	// }
	return nil
}

// func buildFrame(data []byte, frameType FrameType) ([]byte, error) {
// 	headerBuff := []byte{0, 0, 0, 0}

// 	payload := []byte{}

// 	payload = append(payload, data...)

// 	binary.BigEndian.PutUint32(headerBuff, uint32(len(payload)+1))
// 	headerBuff = append(headerBuff, byte(frameType))

// 	frameBuff := []byte{}
// 	frameBuff = append(frameBuff, headerBuff...)
// 	frameBuff = append(frameBuff, payload...)

// 	return frameBuff, nil
// }

func raw(socket *Socket, data []byte, frameType FrameType) {
	if !socket.connected {
		return
	}

	bufferedBytes := 0
	dataLen := len(data)

	// log.Println("raw sending", dataLen, "bytes")

	header := []byte{0, 0, 0, 0, 0, byte(frameType)}

	seq := uint16(socket.sequence.Next())

	binary.BigEndian.PutUint16(header[2:4], seq)

	if dataLen > 0 {
		for bufferedBytes < dataLen {
			headerLen := len(header)

			frameEnd := bufferedBytes + (FRAME_SIZE - headerLen)
			if frameEnd > dataLen {
				frameEnd = bufferedBytes + (dataLen - bufferedBytes)
			}
			dataSlice := data[bufferedBytes:frameEnd]
			sliceLen := len(dataSlice)

			frameLen := sliceLen + headerLen

			binary.BigEndian.PutUint16(header, uint16(frameLen-6))

			buff := []byte{}

			buff = append(buff, header...)
			buff = append(buff, dataSlice...)

			bufferedBytes += sliceLen

			if bufferedBytes == dataLen {
				buff[4] = 2
			} else if bufferedBytes != 0 {
				header[4] = 1
			}

			socket.buffer.append(int(seq), buff)
		}
	} else {
		header[4] = 2
		socket.buffer.append(int(seq), header)
	}
}

func rawBytes(socket *Socket, frame []byte) {
	// log.Println("rawBytes", frame[:6])
	if _, err := socket.connection.Write(frame); err != nil {
		log.Println("rawBytes err", frame[:6], err)
		return
	}
	socket.bytesSent += uint64(len(frame))
}

func send(socket *Socket, event, data string) {
	emit(socket, event, []byte(data))
}

func New(address string) *Socket {
	return &Socket{
		address:    address,
		connection: nil,
		events:     map[string]MessageHandler{},
		connected:  true,
		sequence:   &Sequencer{UpperBoundBits: 16},
	}
}
