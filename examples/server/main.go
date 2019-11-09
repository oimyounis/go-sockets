package main

import (
	"fmt"
	"github.com/oimyounis/go-tcp/server"
	"log"
	"time"
)

func main() {
	srv := server.New(":9090")

	srv.OnConnect(func(socket *server.Socket) {
		log.Printf("socket connected with id: %v\n", socket.Id)

		socket.On("ping", func(socket *server.Socket, data string) {
			log.Println("message received on event: ping: " + data)

			socket.Emit("pong", fmt.Sprintf("%v", time.Now().Unix()))
			socket.Emit("pong2", fmt.Sprintf("%v", time.Now().Unix()))
			socket.Emit("pong3", fmt.Sprintf("%v", time.Now().Unix()))
		})
	})

	srv.OnDisconnect(func(socket *server.Socket) {
		log.Printf("socket disconnected with id: %v\n", socket.Id)
	})

	srv.Start()
}
