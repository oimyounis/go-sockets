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
### Building The Server

**main.go**  
1. Import go-sockets server  
```go
import (
    // ...
    "github.com/oimyounis/go-sockets/server"
    // ...
)
```

2. Initialize a new server instance
```go
srv := server.New(":8000")
```
***New*** takes a single argument, ***address*** that the server will listen on in the format *\<hostname_or_IP\>:\<port\>*. For example: *127.0.0.1:9000*.

3. Set the OnConnect event handler
```go
srv.OnConnection(func(socket *server.Socket) {
    log.Printf("socket connected with id: %v\n", socket.Id)

    socket.On("ping", func(data string) {
        log.Println("message received on event: ping: " + data)

        socket.Send("pong", fmt.Sprintf("%v", time.Now().Unix()))
    })
})
```
Here we setup an event handler that will fire when a client connects to us then we listen for the ***ping*** event and then we send back a ***pong*** event with the current time in seconds.

**Important**: the OnConnect event is a blocking call, so if you have blocking code that does not need to finish before the server starts processing the client's messages you should call it inside a goroutine.

4. Start the server
```go
err := srv.Listen()

if err != nil {
    log.Fatalf("Failed to start server: %v\n", err)
}
```
***Listen*** blocks the current thread listening for connections.

### Building The Client

**main.go**  
1. Import go-sockets client  
```go
import (
    // ...
    "github.com/oimyounis/go-sockets/client"
    // ...
)
```

2. Initialize a new client instance
```go
socket := client.New("localhost:8000")
```
Same as with the server, ***New*** takes a single argument, ***address*** that points to the server's address in the format *\<hostname_or_IP\>:\<port\>*. For example: *127.0.0.1:9000*.

3. Set event handlers
```go
socket.On("connection", func(data string) {
    log.Println("connected to server")

    go func() {
        for {
            socket.Send("ping", fmt.Sprintf("%v", time.Now().Unix()))
            time.Sleep(time.Second)
        }
    }()
})

socket.On("pong", func(data string) {
    log.Printf("pong:%v\n", data)
})
```
Here we setup an event handler that will fire when the client connects to the server and start a goroutine that loops forever sending a ***ping*** event to the server every second then we setup a listener for the ***pong*** event.  

**Important**: notice that we started the loop in a goroutine. That's because the **connection** event is a blocking call, so if you have blocking code that does not need to finish before the client starts listening on the connection you should call it inside a goroutine.

4. Start the client
```go
socket.Listen()

if err != nil {
    log.Fatalf("Couldn't connect to server: %v\n", err)
}
```
***Listen*** blocks the current thread listening for data.

## License
Licensed under the New BSD License.  

## Contribution
All contributions are welcome.  
You are very welcome to submit a new feature, fix a bug or an optimization to the code.  
### To Contribute:
1. Fork this repo.
2. Create a new branch with a discriptive name (example: *feature/some-new-function* or *bugfix/fix-something-somewhere*).
3. Commit and push your code to your new branch.
4. Create a new Pull Request here.  
