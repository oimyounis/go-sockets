package main

import (
	"go-tcp/client/client"
	"log"
	"time"
)

func main() {
	client := client.NewWithEvents("localhost:9090", func(socket *client.Socket) {
		log.Println("connected to server")

		socket.On("remove-user-ack", func(data string) {
			log.Printf("remove-user-ack:%v", data)
		})

		socket.EmitSync("add-user", "id:9,name:omar,age:26,awesome:yes")

		go func() {
			for {
				socket.EmitSync("remove-user", "id:9")
				time.Sleep(time.Second)
			}
		}()
	}, func(socket *client.Socket) {
		log.Println("disconnected from server")
	})

	client.Listen()
}
