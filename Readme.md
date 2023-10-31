To start the server, navigate to the server folder and run the following command:
```jsx
go run server.go -port 5454
```

To run the client, navigate to the client folder and run the follwing command: 
```jsx 
go run client.go -cPort 8080 -sPort 5454
```

To auto generate proto-stuff run the following command. (In this example, the file proto.proto is located in the folder GRPC):
```jsx
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative GRPC/proto.proto
```

To check the ip address:
```jsx
ipconfig getifaddr en0
```