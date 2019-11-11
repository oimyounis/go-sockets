package main

import (
	"fmt"
	"log"
	"time"

	"go-sockets/server"
)

func main() {
	srv, err := server.New(":9090")

	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	srv.OnConnection(func(socket *server.Socket) {
		log.Printf("socket connected with id: %v\n", socket.Id)

		socket.On("ping", func(data string) {
			log.Println("message received on event: ping: " + data)

			socket.Emit("pong", fmt.Sprintf("%v", time.Now().Unix()))
		})

		socket.Broadcast("socket-joined", fmt.Sprintf("a socket joined with id: %v", socket.Id))
	})

	srv.OnDisconnection(func(socket *server.Socket) {
		log.Printf("socket disconnected with id: %v\n", socket.Id)
		socket.Broadcast("socket-left", fmt.Sprintf("a socket left with id: %v", socket.Id))
	})

	srv.Listen()
}
