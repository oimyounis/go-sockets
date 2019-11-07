package main

import (
	"fmt"
	"go-tcp/server/server"
	"log"
	"time"
)

func main() {
	srv := server.NewWithEvents(":9090", func(socket *server.Socket) {
		log.Printf("socket connected with id: %v\n", socket.Id)

		socket.On("add-user", func(socket *server.Socket, data string) {
			log.Println("message received on event: add-user")
			log.Println("with data " + data)
		})

		socket.On("remove-user", func(socket *server.Socket, data string) {
			log.Println("message received on event: remove-user")
			log.Println("with data " + data)
			socket.Emit("remove-user-ack", fmt.Sprintf("%v", time.Now().Unix()))
		})
	}, func(socket *server.Socket) {
		log.Printf("socket disconnected with id: %v\n", socket.Id)
	})

	srv.Start()
}
