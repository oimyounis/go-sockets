package main

import (
	"fmt"
	"log"
	"time"

	"go-sockets/client"
)

func main() {
	socket, err := client.New("localhost:9090")

	if err != nil {
		log.Fatalf("Couldn't connect to server: %v", err)
	}

	socket.On("connection", func(_ string) {
		log.Println("connected to server")

		go func() {
			for {
				socket.EmitSync("ping", fmt.Sprintf("%v", time.Now().Unix()))
				time.Sleep(time.Second)
			}
		}()
	})

	socket.On("pong", func(data string) {
		log.Printf("pong:%v", data)
	})

	socket.On("socket-joined", func(data string) {
		log.Printf("FROM SERVER: %v\n", data)
	})

	socket.On("socket-left", func(data string) {
		log.Printf("FROM SERVER: %v\n", data)
	})

	socket.On("disconnection", func(_ string) {
		log.Println("disconnected from server")
	})

	socket.Listen()
}
