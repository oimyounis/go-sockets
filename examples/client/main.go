package main

import (
	"fmt"
	"github.com/oimyounis/go-tcp/client"
	"log"
	"time"
)

func main() {
	c := client.New("localhost:9090")

	c.OnConnect(func(socket *client.Socket) {
		log.Println("connected to server")

		socket.On("pong", func(data string) {
			log.Printf("pong:%v", data)
		})
		socket.On("pong2", func(data string) {
			log.Printf("pong2:%v", data)
		})
		socket.On("pong3", func(data string) {
			log.Printf("pong3:%v", data)
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
