package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	"tritontube/internal/proto"
	"tritontube/internal/storage"
)

func main() {
	host := flag.String("host", "localhost", "Host address for the server")
	port := flag.Int("port", 8090, "Port number for the server")
	flag.Parse()

	// Validate arguments
	if *port <= 0 {
		panic("Error: Port number must be positive")
	}

	if flag.NArg() < 1 {
		fmt.Println("Usage: storage [OPTIONS] <baseDir>")
		fmt.Println("Error: Base directory argument is required")
		return
	}
	baseDir := flag.Arg(0)

	addr := fmt.Sprintf("%s:%d", *host, *port)
	log.Printf("[STARTING] Storage server on %s", addr)

	// Create gRPC listener
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("[FATAL] Failed to listen: %v", err)
	}

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Create and register your server
	server := &storage.Server{BaseDir: baseDir}
	proto.RegisterStorageServer(grpcServer, server)

	// Serve
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("[FATAL] Failed to serve: %v", err)
	}
}
