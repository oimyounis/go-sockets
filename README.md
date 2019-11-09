# go-sockets
go-sockets is an event-driven TCP sockets framework that allows you to create a server/client architecture with ease based on named events.

## Project Status
go-sockets is currently in an early development stage and is not ready for production use, yet.

## Installation
### Dependencies
Execute the following command in your Terminal:
```bash
go get -u github.com/google/uuid
```

### Download go-sockets
Execute the following command in your Terminal:
```bash
go get -u github.com/oimyounis/go-sockets
```

## Hello World
### Building the server

**main.go**  
1. Import go-sockets server  
```go
import (
    "github.com/oimyounis/go-sockets/server"
)
```

2. Initialize a new server instance
```go
srv := server.New(":9090")
```
***New*** takes a single argument, ***address*** that the server will listen on in the format *\<hostname_or_IP\>:\<port\>*. For example: *127.0.0.1:9000*.

3. Set the OnConnect event handler
```go
srv.OnConnect(func(socket *server.Socket) {
    log.Printf("socket connected with id: %v\n", socket.Id)

    socket.On("ping", func(socket *server.Socket, data string) {
        log.Println("message received on event: ping: " + data)

        socket.Emit("pong", fmt.Sprintf("%v", time.Now().Unix()))
    })
})
```
Here we setup an event handler that will fire when a client connects to us then we listen for the ***ping*** event and then we send back a ***pong*** event with the current time in seconds.

4. Start the server
```go
srv.Start()
```
***Start*** blocks the current thread listening for connections.

### Building the client

**main.go**  
1. Import go-sockets client  
```go
import (
    "github.com/oimyounis/go-sockets/client"
)
```

2. Initialize a new client instance
```go
c := client.New("localhost:9090")
```
Same as with the server, ***New*** takes a single argument, ***address*** that points to the server's address in the format *\<hostname_or_IP\>:\<port\>*. For example: *127.0.0.1:9000*.

3. Set the OnConnect event handler
```go
c.OnConnect(func(socket *client.Socket) {
    log.Println("connected to server")

    socket.On("pong", func(data string) {
        log.Printf("pong:%v", data)
    })

    go func() {
        for {
            socket.Emit("ping", fmt.Sprintf("%v", time.Now().Unix()))
            time.Sleep(time.Second)
        }
    }()
})
```
Here we setup an event handler that will fire when the client connects to the server then we listen for the ***pong*** event then we start a goroutine that loops forever sending a ***ping*** event to the server every second.  

**Important**: notice that we started the loop in a goroutine. That's because the OnConnect is a blocking call, so if you have blocking code that does not need to finish before the client starts listening on the connection you should call it inside a goroutine.

4. Start the client
```go
c.Listen()
```
***Start*** blocks the current thread listening for data.

## Contribution
All contributions are welcome.  
You are very welcome to submit a new feature, fix a bug or an optimization to the code.  
