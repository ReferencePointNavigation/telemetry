package main

//go:generate protoc -I ../../protobuf --go_out=plugins=grpc:../../protobuf ../../protobuf/*.proto

import (
	"flag"
	"fmt"
	db2 "github.com/ReferencePointNavigation/telemetry/db"
	pb "github.com/ReferencePointNavigation/telemetry/protobuf"
	"github.com/ReferencePointNavigation/telemetry/telemetry"
	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/testdata"
	"log"
	"net"
	"os"
	"os/signal"
)

var (
	tls = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	certFile = flag.String("cert_file", "", "The TLS cert file")
	keyFile = flag.String("key_file", "", "The TLS key file")
	port = flag.Int("port", 10000, "The server port")
	dbFile = flag.String("database", "./telemetry.db", "path to telemetry database")
)

// Get preferred outbound ip of this machine
func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

func main() {

	flag.Parse()

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	var opts []grpc.ServerOption
	if *tls {
		if *certFile == "" {
			*certFile = testdata.Path("server1.pem")
		}
		if *keyFile == "" {
			*keyFile = testdata.Path("server1.key")
		}
		creds, err := credentials.NewServerTLSFromFile(*certFile, *keyFile)
		if err != nil {
			log.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}

	db, err := db2.OpenDatabase(*dbFile)

	grpcServer := grpc.NewServer(opts...)

	server, err := telemetry.NewServer(db)

	pb.RegisterTelemetryServer(grpcServer, server)

	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan struct{})
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		<-signalChan
		fmt.Println("\nReceived an interrupt, stopping services...")
		grpcServer.GracefulStop()
		close(cleanupDone)
	}()

	log.Printf("Starting telemtry server on %s:%d", GetOutboundIP(), *port)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}

	<-cleanupDone

}
