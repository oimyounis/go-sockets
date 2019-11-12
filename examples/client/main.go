package main

import (
	"log"

	"go-sockets/client"
)

func main() {
	socket, err := client.New("localhost:9090")

	if err != nil {
		log.Fatalf("Couldn't connect to server: %v", err)
	}

	socket.On("connection", func(_ string) {
		log.Println("connected to server")

		// go func() {
		// 	for {
		size := 15555555
		buff := make([]byte, size)
		for i := 0; i < size; i++ {
			buff[i] = byte(i)
		}

		socket.Emit("test", buff)

		// socket.Send("test2", string(bytes.Repeat([]byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z'}, 445123)))
		// socket.EmitSync("test", string([]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
		// 	time.Sleep(time.Second)
		// }
		// }()
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
