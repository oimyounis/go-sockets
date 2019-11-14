package main

import (
	"bytes"
	"log"
	"strconv"
	"time"

	"go-sockets/client"
)

func mbToInt(size string) int {
	num := size[:len(size)-1]
	conv, _ := strconv.ParseFloat(num, 64)
	switch string(size[len(size)-1]) {
	case "b":
		return int(conv)
	case "k":
		return int(conv * 1024)
	case "m":
		return int(conv * 1024 * 1024)
	case "g":
		return int(conv * 1024 * 1024 * 1024)
	}

	return 0
}

func mbSlice(size string) []byte {
	return bytes.Repeat([]byte{1}, mbToInt(size))
}

func main() {
	socket, err := client.New("localhost:9090")

	if err != nil {
		log.Fatalf("Couldn't connect to server: %v", err)
	}

	go func() {
		for {
			time.Sleep(time.Second * 10)
			log.Println("total sent bytes:", socket.TotalSentBytes)
		}
	}()

	socket.On("connection", func(_ string) {
		log.Println("connected to server")

		go func() {
			for {
				socket.Emit("test111", bytes.Repeat([]byte{2}, 123456))
				socket.Emit("test2", bytes.Repeat([]byte{2}, 1234))
				time.Sleep(time.Millisecond * 100)
			}
		}()

		// socket.Send("test2", string(bytes.Repeat([]byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z'}, 445123)))
		// socket.EmitSync("test", string([]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
		// 	time.Sleep(time.Second)
		// }
		// }()
	})

	socket.On("testee", func(data string) {
		// log.Println("testee", len(data))
	})
	socket.On("testee2", func(data string) {
		// log.Println("testee2", len(data))
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
		log.Println("disconnection: disconnected from server")
	})

	socket.Listen()
}
