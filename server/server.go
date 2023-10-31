package main

import (
	"context"
	"fmt"
	proto "grpc/GRPC" // Update this import path as needed
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
)

type Connection struct {
	stream proto.Broadcast_CreateStreamServer
	id     string
	name   string
	active bool
	error  chan error
}

type Server struct {
	Connections []*Connection
	proto.UnimplementedBroadcastServer
	timestamp int64
}

func (s *Server) CreateStream(pconn *proto.Connect, stream proto.Broadcast_CreateStreamServer) error {
	s.timestamp += 1 // Update timestamp after receiving connection request
	log.Printf("[%d] Info received: New client want to connect: %s \n", s.timestamp, pconn.User.Name)
	fmt.Printf("[%d] Info received: New client want to connect: %s \n", s.timestamp, pconn.User.Name)

	conn := &Connection{
		stream: stream,
		id:     pconn.User.Id,
		name:   pconn.User.Name,
		active: true,
		error:  make(chan error),
	}

	s.Connections = append(s.Connections, conn)
	s.timestamp += 1 // Update timestamp before sending join message

	joinMessage := &proto.Message{
		Id:        conn.id,
		Name:      "(Server) " + conn.name,
		Content:   fmt.Sprintf("%s joined the server", conn.name),
		Timestamp: s.timestamp,
	}

	log.Printf("[%d] Info send: New client connected: %s \n", joinMessage.Timestamp, conn.name)
	fmt.Printf("[%d] Info send: New client connected: %s \n", joinMessage.Timestamp, conn.name)

	for _, c := range s.Connections {
		if c.active {
			err := c.stream.Send(joinMessage) //Send join message to all active clients
			if err != nil {
				log.Printf("Error sending join message to %s: %v \n", c.name, err)
				fmt.Printf("Error sending join message to %s: %v \n", c.name, err)
			}
		}
	}

	go func() {
		<-stream.Context().Done()
		conn.active = false
		conn.error <- nil

		s.timestamp += 1 // Update timestamp before sending disconnection message
		disconnectionMessage := &proto.Message{
			Id:        conn.id,
			Name:      "(Server) " + conn.name,
			Content:   "disconnected from server",
			Timestamp: s.timestamp,
		}

		log.Printf("[%d] Info send: User %s disconnected \n", disconnectionMessage.Timestamp, conn.name)
		fmt.Printf("[%d] Info send: User %s disconnected \n", disconnectionMessage.Timestamp, conn.name)

		for _, c := range s.Connections {
			if c.active && c.id != conn.id {
				err := c.stream.Send(disconnectionMessage) // Send disconnection message to other clients
				if err != nil {
					log.Printf("Error sending disconnection message to %s: %v \n", c.name, err)
					fmt.Printf("Error sending disconnection message to %s: %v \n", c.name, err)
				}
			}
		}

		// Remove disconnected client
		s.removeDisconnectedClients()
	}()

	return <-conn.error
}

func (s *Server) BroadcastMessage(ctx context.Context, msg *proto.Message) (*proto.Close, error) {
	s.timestamp = max(s.timestamp, msg.Timestamp) + 1 // Update timestamp after receiving message
	log.Printf("[%d] Message received from %s: %s \n", s.timestamp, msg.Name, msg.Content)
	fmt.Printf("[%d] Message received from %s: %s \n", s.timestamp, msg.Name, msg.Content)

	// Send the message to all active clients except the sender
	senderName := ""
	var recipients []string

	for _, conn := range s.Connections {
		if conn.id == msg.Id {
			senderName = conn.name
		}
	}
	s.timestamp += 1            // Update timestamp before sending
	msg.Timestamp = s.timestamp // Update message timestamp before sending
	for _, conn := range s.Connections {
		if conn.active && conn.id != msg.Id {
			err := conn.stream.Send(msg) // send message
			if err != nil {
				log.Printf("Error with Stream: %v - Error: %v \n", conn.stream, err)
				fmt.Printf("Error with Stream: %v - Error: %v \n", conn.stream, err)
				conn.active = false
				conn.error <- err
			} else {
				recipients = append(recipients, conn.name)
			}
		}
	}

	log.Printf("[%d] Message from %s sent to %v: %s \n", s.timestamp, senderName, recipients, msg.Content) // Log the message sent
	fmt.Printf("[%d] Message from %s sent to %v: %s \n", s.timestamp, senderName, recipients, msg.Content)

	return &proto.Close{}, nil
}

func (s *Server) removeDisconnectedClients() {
	// Remove disconnected clients
	var activeConnections []*Connection
	for _, conn := range s.Connections {
		if conn.active {
			activeConnections = append(activeConnections, conn)
		}
	}
	s.Connections = activeConnections
}

func main() {
	f, err := os.OpenFile("../logs/serverlog.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v \n", err)
		fmt.Printf("error opening file: %v \n", err)
	}
	defer f.Close()

	log.SetOutput(f)

	var connections []*Connection
	server := &Server{Connections: connections, timestamp: 0}

	grpcServer := grpc.NewServer()
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("error creating the server %v \n", err)
		fmt.Printf("error creating the server %v \n", err)
	}
	server.timestamp += 1
	fmt.Printf("[%d] Starting server at port :8080\n", server.timestamp)
	log.Printf("[%d] Starting server at port :8080\n", server.timestamp)

	proto.RegisterBroadcastServer(grpcServer, server)

	// Handle interrupt signal to log client disconnections
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		for _, conn := range server.Connections {
			fmt.Printf("User %s disconnected ***** \n", conn.name)
			log.Printf("User %s disconnected ***** \n", conn.name)
		}
		os.Exit(1)
	}()

	grpcServer.Serve(listener)
}
