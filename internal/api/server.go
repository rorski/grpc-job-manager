package api

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/rorski/grpc-job-manager/internal/job"
	"github.com/rorski/grpc-job-manager/worker"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Config holds information for setting up a gRPC server (host, port and certificates)
type Config struct {
	Host                 string
	Port                 int
	Certificate, Key, CA string
}

func setupCreds(certFile, keyFile, caFile string) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load x509 key pair: %v", err)
	}
	caPem, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA pem: %v", err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caPem) {
		return nil, fmt.Errorf("failed to add CA cert to pool: %v", err)
	}

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert, // require client auth (i.e., mTLS)
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS13,
	}), nil
}

func newGrpcServer(conf Config, creds credentials.TransportCredentials) (*grpc.Server, net.Listener, error) {
	address := fmt.Sprintf("%s:%d", conf.Host, conf.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to listen on %s: %v", address, err)
	}
	server := grpc.NewServer(
		grpc.Creds(creds),
		grpc.UnaryInterceptor(unaryInterceptor), // unary interceptor to verify client access to methods
	)

	return server, listener, nil
}

// Serve creates a new gRPC server from a Config
func Serve(conf Config) error {
	creds, err := setupCreds(conf.Certificate, conf.Key, conf.CA)
	if err != nil {
		return fmt.Errorf("error setting up credentials: %v", err)
	}
	s, lis, err := newGrpcServer(conf, creds)
	if err != nil {
		return fmt.Errorf("error creating new grpc server: %v", err)
	}
	defer lis.Close()
	job.RegisterJobManagerServer(s, &jobManagerServer{Worker: *worker.New()})

	// just using the standard "log" library. In production this would be something more robust like logrus or zap
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}

	// shutdown gracefully
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-shutdown
		s.GracefulStop()
	}()

	return nil
}
