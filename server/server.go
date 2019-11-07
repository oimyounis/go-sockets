package server

import (
	"bufio"
	"log"
	"net"
	"strings"
)

type Socket struct {
	connection net.Conn
	events     []string
}

type Server struct {
	listener net.Listener
	sockets  []Socket
}

func (s *Server) Start() {
	defer s.listener.Close()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			log.Printf("Couldn't accept connection: %v\n", err)
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	log.Printf("Accepted connection from %v\n", conn.RemoteAddr().String())
	for {
		recv, err := bufio.NewReader(conn).ReadString('~')
		if err != nil {
			log.Printf("A connection was closed due to an error: %v", err)
			break
		}

		str := strings.TrimSpace(recv)
		log.Println(str)
	}
	conn.Close()
}

func New(network string, address string) *Server {
	l, err := net.Listen(network, address)
	if err != nil {
		log.Fatalf("Failed to start server: %v\n", err)
	}
	s := &Server{listener: l}
	return s
}
