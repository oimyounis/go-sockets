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

		go func() {
			for {
				// socket.EmitSync("test11", bytes.Repeat([]byte{2}, mbToInt("3m")))

				buff := []byte{}
				for i := 0; i < mbToInt("100k"); i++ {
					buff = append(buff, byte(i))
				}

				buff2 := []byte{}
				for i := 0; i < mbToInt("500k"); i++ {
					buff2 = append(buff2, byte(i))
				}

				socket.Emit("test2", buff)
				socket.Emit("test2", buff2)
				time.Sleep(time.Second * 5)
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
