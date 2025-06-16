package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"tritontube/internal/web"
)

func main() {
	host := flag.String("host", "localhost", "host to listen on")
	port := flag.Int("port", 8080, "port to listen on")
	flag.Parse()

	args := flag.Args()
	if len(args) != 4 {
		fmt.Println("Usage: ./main -host <host> -port <port> <METADATA_TYPE> <METADATA_OPTIONS> <CONTENT_TYPE> <CONTENT_OPTIONS>")
		os.Exit(1)
	}

	metadataType := args[0]
	metadataOpt := args[1]
	contentType := args[2]
	contentOpt := args[3]

	var metadata web.VideoMetadataService
	var err error

	switch metadataType {
	case "sqlite":
		metadata, err = web.NewSQLiteVideoMetadataService(metadataOpt)
	case "etcd":
		metadata, err = web.NewEtcdVideoMetadataService(metadataOpt)
	default:
		log.Fatalf("Unsupported metadata type: %s", metadataType)
	}
	if err != nil {
		log.Fatalf("Failed to create metadata service: %v", err)
	}

	var content web.VideoContentService
	switch contentType {
	case "fs":
		if err := os.MkdirAll(contentOpt, os.ModePerm); err != nil {
			log.Fatalf("failed to create directory %s: %v", contentOpt, err)
		}
		content = web.NewFSVideoContentService(contentOpt)
	case "nw":
		parts := strings.Split(contentOpt, ",")
		if len(parts) < 2 {
			log.Fatalf("Invalid CONTENT_OPTIONS for nw: must be in form adminhost:adminport,node1:port1,node2:port2,...")
		}
		adminHostPort := parts[0]
		nodeAddrs := parts[1:]

		nwContent, err := web.NewNetworkVideoContentService(nodeAddrs, adminHostPort)
		if err != nil {
			log.Fatalf("Failed to initialize NetworkVideoContentService: %v", err)
		}
		content = nwContent

	default:
		log.Fatalf("Unsupported content type: %s", contentType)
	}

	srv := web.NewServer(metadata, content)
	addr := fmt.Sprintf("%s:%d", *host, *port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}
	log.Printf("TritonTube running at http://%s\n", addr)

	if err := srv.Start(listener); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
