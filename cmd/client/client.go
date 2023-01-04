package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/rorski/grpc-job-manager/internal/job"
)

type clientCerts struct {
	CertPool          *x509.CertPool
	ClientCertificate tls.Certificate
}

func loadCerts(ctx *cli.Context) (*clientCerts, error) {
	caPem, err := os.ReadFile(ctx.String("ca"))
	if err != nil {
		return nil, fmt.Errorf("failed to read ca.pem file: %v", err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caPem) {
		return nil, fmt.Errorf("failed to add CA cert to pool: %v", err)
	}
	// Load client's certificate and private key
	clientCert, err := tls.LoadX509KeyPair(ctx.String("cert"), ctx.String("key"))
	if err != nil {
		return nil, fmt.Errorf("failed to load the client certificates")
	}

	return &clientCerts{certPool, clientCert}, nil
}

// NewClient creates and returns a new cli.App object to be run by app.Run.
// It uses a cli.BeforeFunc (https://pkg.go.dev/github.com/urfave/cli#BeforeFunc) to create
// the grpc connection from context values (i.e., command line paramters), then a cli.AfterFunc
// (https://pkg.go.dev/github.com/urfave/cli#AfterFunc) to tear it down.
func NewClient() (app *cli.App, err error) {
	var (
		conn      *grpc.ClientConn
		jobClient job.JobManagerClient
	)

	app = cli.NewApp()
	commands := []*cli.Command{
		{
			Name:  "start",
			Usage: "start a job",
			Action: func(c *cli.Context) error {
				if err = Start(jobClient, c); err != nil {
					log.Fatalf("failed starting job: %v", err)
				}
				return nil
			},
		},
		{
			Name:      "stop",
			Usage:     "stop a job",
			UsageText: "client stop [uuid]",
			Action: func(c *cli.Context) error {
				if err = Stop(jobClient, c); err != nil {
					log.Fatalf("Error stopping job: %v", err)
				}
				return nil
			},
		},
		{
			Name:      "status",
			Usage:     "get status of a job",
			UsageText: "client status [uuid]",
			Action: func(c *cli.Context) error {
				if err = Status(jobClient, c); err != nil {
					log.Fatalf("Error getting status: %v", err)
				}
				return nil
			},
		},
		{
			Name:      "output",
			Usage:     "stream output of a job",
			UsageText: "client output [uuid]",
			Action: func(c *cli.Context) error {
				if err = Output(jobClient, c); err != nil {
					log.Fatalf("Error streaming output: %v", err)
				}
				return nil
			},
		},
	}
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:  "host",
			Usage: "gRPC host address",
			Value: "localhost",
		},
		&cli.UintFlag{
			Name:  "port",
			Usage: "gRPC port",
			Value: 31234,
		},
		&cli.StringFlag{
			Name:  "ca",
			Usage: "path to CA certificate",
			Value: "./certs/ca.pem",
		},
		&cli.StringFlag{
			Name:  "cert",
			Usage: "path to client TLS certificate",
			Value: "./certs/client_admin.pem",
		},
		&cli.StringFlag{
			Name:  "key",
			Usage: "path to client TLS key",
			Value: "./certs/client_admin.key",
		},
	}
	// set up grpc connection before executing commands
	app.Before = func(ctx *cli.Context) error {
		certs, err := loadCerts(ctx)
		if err != nil {
			log.Fatalf("error loading client cert: %v", err)
		}

		address := fmt.Sprintf("%s:%d", ctx.String("host"), ctx.Int("port"))
		conn, err = grpc.DialContext(ctx.Context, address, grpc.WithTransportCredentials(
			credentials.NewTLS(&tls.Config{
				Certificates: []tls.Certificate{certs.ClientCertificate},
				RootCAs:      certs.CertPool,
			}),
		))
		if err != nil {
			log.Fatalf("error connecting to %s: %v", address, err)
		}
		jobClient = job.NewJobManagerClient(conn)

		return nil
	}
	// tear down grpc connection after running client
	app.After = func(ctx *cli.Context) error {
		if err = conn.Close(); err != nil {
			return fmt.Errorf("error closing grpc client connection: %v", err)
		}
		return nil
	}
	app.Commands = commands
	app.Flags = flags
	app.Name = "client"
	app.Usage = "grpc job manager client"

	return app, nil
}

func main() {
	app, err := NewClient()
	if err != nil {
		log.Fatal(err)
	}

	if err = app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
