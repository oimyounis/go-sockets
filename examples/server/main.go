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

	srv.OnConnect(func(socket *server.Socket) {
		log.Printf("socket connected with id: %v\n", socket.Id)

		socket.On("ping", func(data string) {
			log.Println("message received on event: ping: " + data)

			socket.Emit("pong", fmt.Sprintf("%v", time.Now().Unix()))
		})
	})

	srv.OnDisconnect(func(socket *server.Socket) {
		log.Printf("socket disconnected with id: %v\n", socket.Id)
	})

	srv.Start()
}
