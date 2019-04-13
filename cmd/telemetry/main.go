package main

//go:generate protoc -I ../../protobuf --go_out=plugins=grpc:../../protobuf ../../protobuf/*.proto

import (
	"flag"
	"fmt"
	pb "github.com/ReferencePointNavigation/telemetry/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/testdata"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
)

var (
	tls        = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	certFile   = flag.String("cert_file", "", "The TLS cert file")
	keyFile    = flag.String("key_file", "", "The TLS key file")
	port       = flag.Int("port", 10000, "The server port")
)

type particleCastServer struct {

	mu sync.Mutex

}

func (p *particleCastServer) CastParticleState(stream pb.ParticleCast_CastParticleStateServer) error {
	var pointCount int32

	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.CastSummary{
				NumStates:   pointCount,
			})
		}
		if err != nil {
			return err
		}
		pointCount++
	}
}

func newServer() *particleCastServer {
	p := &particleCastServer{}
	return p
}

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
	grpcServer := grpc.NewServer(opts...)


	pb.RegisterParticleCastServer(grpcServer, newServer())

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
		log.Fatal("Error starting server: %s", err)
	}

	<-cleanupDone

}
