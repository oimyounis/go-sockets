package main

import (
	"tcp/server"
)

func main() {
	srv := server.New("tcp4", ":9090")

	srv.Start()
}
