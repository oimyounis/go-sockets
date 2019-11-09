package main

import (
	"fmt"
	"log"
	"time"

	"go-sockets/client"
)

func main() {
	c, err := client.New("localhost:9090")

	if err != nil {
		log.Fatalf("Couldn't connect to server: %v", err)
	}

	c.OnConnect(func(socket *client.Socket) {
		log.Println("connected to server")

		socket.On("pong", func(data string) {
			log.Printf("pong:%v", data)
		})

		go func() {
			for {
				socket.EmitSync("ping", fmt.Sprintf("%v", time.Now().Unix()))
				time.Sleep(time.Second)
			}
		}()
	})

	c.OnDisconnect(func(socket *client.Socket) {
		log.Println("disconnected from server")
	})

	c.Listen()
}
