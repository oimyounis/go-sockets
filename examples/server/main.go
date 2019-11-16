package main

import (
	"bytes"
	"log"
	"strconv"

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
	srv := server.New(":9090")

	// go func() {
	// 	for {
	// 		time.Sleep(time.Second * 10)
	// 		log.Println("total sent bytes:", srv.TotalSentBytes)
	// 	}
	// }()

	srv.OnConnection(func(socket *server.Socket) {
		log.Printf("socket connected with id: %v\n", socket.Id)

		socket.On("test11", func(data string) {
			c := bytes.Count([]byte(data), []byte{2})
			if c != mbToInt("3m") {
				log.Fatalln("test11 len mismatch", c)
			}
			log.Println("test11:", c)
		})
		socket.On("test2", func(data string) {
			c := bytes.Count([]byte(data), []byte{3})
			if c != mbToInt("200k") {
				log.Fatalln("test2 len mismatch", c)
			}
			log.Println("test2:", c)
		})
	})

	srv.OnDisconnection(func(socket *server.Socket) {
		log.Printf("socket disconnected with id: %v\n", socket.Id)
	})

	err := srv.Listen()

	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
