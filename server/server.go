package server

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net"
	"strings"
	"sync"
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

type ConnectionHandler func(socket *Socket)
type MessageHandler func(data string)

type Socket struct {
	Id               string
	connection       net.Conn
	events           map[string]MessageHandler
	server           *Server
	connected        bool
	lastHeartbeatAck int64
	// mutex            sync.Mutex
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

func (s *Socket) SendSync(event, data string) {
	send(s, event, data)
}

func (s *Socket) Send(event, data string) {
	go send(s, event, data)
}

func (s *Socket) Emit(event string, data []byte) {
	go emit(s, event, data)
}

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
	s.connected = false
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
			log.Println("disconnecting client")
			s.connected = false
			break
		}
	}
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

	batchQueue := make(map[uint32][]byte)

	// var mutex sync.Mutex

	for {
		if !s.connected {
			break
		}

		frame := make([]byte, FRAME_SIZE)
		n, err := io.ReadFull(sockBuffer, frame)
		if err != nil {
			// log.Println(err)
			break
		}

		// log.Printf("in [%v] > %v", n, frame)

		// go func(frame []byte) {
		if !s.connected {
			return
		}
		frameLen := len(frame)

		if frameLen > 0 {
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
					}

					eventName := string(frame[8:eventEnd])
					// log.Println("eventLen", eventLen, "eventName", eventName)

					dataLen := binary.BigEndian.Uint16(frame[eventEnd : eventEnd+2])

					data := frame[eventEnd+2 : eventEnd+2+dataLen]

					// mutex.Lock()
					batchQueue[batchId] = append(batchQueue[batchId], data...)
					// mutex.Unlock()

					if isLast {
						go s.envokeEvent(eventName, string(batchQueue[batchId]))
						// mutex.Lock()
						delete(batchQueue, batchId)
						// mutex.Unlock()
					}
				}
			case byte(FRAME_TYPE_HEARTBEAT):
				log.Println("hb in >")
				raw(s, []byte{}, FRAME_TYPE_HEARTBEAT_ACK)
			case byte(FRAME_TYPE_HEARTBEAT_ACK):
				s.lastHeartbeatAck = time.Now().UnixNano() / 1000000
			}
		}
		// }(buff)
	}
	s.connection.Close()
	s.server.removeSocket(s)
	s.server.disconnectEvent(s)
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
	frame = pad(frame, FRAME_SIZE)

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
