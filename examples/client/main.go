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
	socket := client.New("localhost:9090")

	// go func() {
	// 	for {
	// 		time.Sleep(time.Second * 10)
	// 		log.Println("total sent bytes:", socket.TotalSentBytes)
	// 	}
	// }()

	socket.On("connection", func(_ string) {
		log.Println("connected to server")

		buff := []byte{}
		for i := 0; i < mbToInt("1k"); i++ {
			buff = append(buff, byte(i))
		}

		buff2 := []byte{}
		for i := 0; i < mbToInt("3k"); i++ {
			buff2 = append(buff2, byte(i))
		}

		buff3 := []byte{}
		for i := 0; i < mbToInt("4k"); i++ {
			buff3 = append(buff3, byte(i))
		}

		buff4 := []byte{}
		for i := 0; i < mbToInt("7k"); i++ {
			buff4 = append(buff4, byte(i))
		}

		go func() {
			for {
				// socket.EmitSync("test11", bytes.Repeat([]byte{2}, mbToInt("3m")))

				socket.Emit("test4", buff2)
				socket.Emit("test2", buff)
				socket.Emit("test5", buff4)
				socket.Emit("test3", buff3)

				time.Sleep(time.Second * 1)
			}
		}()
	})

	socket.On("disconnection", func(_ string) {
		log.Println("disconnection: disconnected from server")
	})

	err := socket.Listen()

	if err != nil {
		log.Fatalf("Couldn't connect to server: %v", err)
	}
}
