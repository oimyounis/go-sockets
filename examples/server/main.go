package main

import (
	"bytes"
	"log"
	"strconv"
	"time"

	"go-sockets/server"
)

func mbToInt(size string) int {
	num := size[:len(size)-1]
	conv, _ := strconv.Atoi(num)
	switch string(size[len(size)-1]) {
	case "b":
		return conv
	case "k":
		return conv * 1024
	case "m":
		return conv * 1024 * 1024
	case "g":
		return conv * 1024 * 1024 * 1024
	}

	return 0
}

func mbSlice(size string) []byte {
	return bytes.Repeat([]byte{1}, mbToInt(size))
}

func main() {
	srv, err := server.New(":9090")

	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	go func() {
		for {
			time.Sleep(time.Second * 10)
			// log.Println("total sent bytes:", srv.TotalSentBytes)
		}
	}()

	srv.OnConnection(func(socket *server.Socket) {
		log.Printf("socket connected with id: %v\n", socket.Id)

		go func() {
			// for {
			// socket.Emit("testee", mbSlice("20m"))
			// socket.Emit("testee2", mbSlice("500k"))
			// time.Sleep(time.Millisecond * 2)
			// }
		}()

		socket.On("test111", func(data string) {
			if len(data) != 123456 {
				log.Fatalln("test111 len(data) != 123456", len(data))
			}
			log.Println("test111:", len(data))
		})
		socket.On("test2", func(data string) {
			if len(data) != 1234 {
				log.Fatalln("test2 len(data) != 1234", len(data))
			}
			log.Println("test2:", len(data))
		})

		// socket.On("ping", func(data string) {
		// 	log.Println("message received on event: ping: " + data)

		// 	socket.Emit("pong", fmt.Sprintf("wfwefwef\n\n%v", time.Now().Unix()))
		// })

		// socket.Broadcast("socket-joined", fmt.Sprintf("a socket joined with id: %v", socket.Id))
	})

	srv.OnDisconnection(func(socket *server.Socket) {
		log.Printf("socket disconnected with id: %v\n", socket.Id)
		// socket.Broadcast("socket-left", fmt.Sprintf("a socket left with id: %v", socket.Id))
	})

	srv.Listen()
}
