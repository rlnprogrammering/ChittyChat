package main

import (
	"bufio"
	"crypto/sha256"
	"flag"
	"fmt"
	proto "grpc/GRPC" // Update this import path as needed
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"encoding/hex"
	"log"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var client proto.BroadcastClient
var wait *sync.WaitGroup
var timestamp int64

func init() {
	wait = &sync.WaitGroup{}
}

func connect(user *proto.User) error {
	var streamerror error

	stream, err := client.CreateStream(context.Background(), &proto.Connect{
		User:   user,
		Active: true,
	})
	timestamp += 1
	log.Printf("[%d] %s joined the server \n", timestamp, user.Name)
	fmt.Printf("[%d] %s joined the server \n", timestamp, user.Name)

	if err != nil {
		return fmt.Errorf("connection failed: %v \n", err)
	}

	wait.Add(1)
	go func(str proto.Broadcast_CreateStreamClient) {
		defer wait.Done()

		for {
			msg, err := str.Recv() // Receive message from server
			if err != nil {
				streamerror = fmt.Errorf("Error reading message: %v \n", err)
				break
			}
			timestamp = max(timestamp, msg.Timestamp) + 1
			msg.Timestamp = timestamp
			log.Printf("[%d] %s : %s\n", msg.Timestamp, msg.Name, msg.Content) // Log message to textfile
			fmt.Printf("[%d] %s : %s\n", msg.Timestamp, msg.Name, msg.Content) // Print message to client
		}
	}(stream)

	return streamerror
}

func main() {
	timestamp = int64(0)
	var disconnectionOnce sync.Once

	var name string
	flag.StringVar(&name, "N", "Anon", "The name of the user")
	flag.Parse()

	id := sha256.Sum256([]byte(time.Now().String() + name))

	// Log in textfile
	filepath := "../logs/clientlog-" + name + ".txt"
	f, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v \n", err)
		fmt.Printf("error opening file: %v \n", err)
	}
	defer f.Close()

	log.SetOutput(f)

	// Handle interrupt signal to log client disconnections and send a disconnection message
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() { //log and handle disconnection
		<-c
		disconnectionOnce.Do(func() {
			log.Printf("(You) %s left the server \n", name)
			fmt.Printf("(You) %s left the server \n", name)
		})
		os.Exit(1)
	}()

	conn, err := grpc.Dial("localhost:8080", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Couldn't connect to service: %v \n", err)
		fmt.Printf("Couldn't connect to service: %v \n", err)
	}

	client = proto.NewBroadcastClient(conn)
	user := &proto.User{
		Id:   hex.EncodeToString(id[:]),
		Name: name,
	}

	connect(user)

	wait.Add(1)
	go func() {
		defer wait.Done()

		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			timestamp += 1
			msg := &proto.Message{
				Id:        user.Id,
				Name:      "(Client) " + user.Name,
				Content:   scanner.Text(),
				Timestamp: timestamp,
			}

			log.Printf("[%d] %s (You): %s\n", msg.Timestamp, msg.Name, msg.Content)
			fmt.Printf("[%d] %s (You): %s\n", msg.Timestamp, msg.Name, msg.Content)

			_, err := client.BroadcastMessage(context.Background(), msg) // send message
			if err != nil {
				fmt.Printf("Error Sending Message: %v \n", err)
				os.Exit(1)
			}
		}
	}()

	// Block and keep the client running until manually disconnected
	wait.Wait()
}
