package main

import (
	"fmt"
	"log"
	"os"

	"github.com/rorski/grpc-job-manager/internal/api"
	"github.com/rorski/grpc-job-manager/worker"

	"github.com/urfave/cli/v2" // imports as package "cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "server"
	app.Usage = "grpc job manager server"
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "cert",
			Usage: "path to certificate",
			Value: "./certs/server.pem",
		},
		&cli.StringFlag{
			Name:  "key",
			Usage: "path to key",
			Value: "./certs/server.key",
		},
		&cli.StringFlag{
			Name:  "ca",
			Usage: "path to CA certificate",
			Value: "./certs/ca.pem",
		},
		&cli.IntFlag{
			Name:  "port",
			Usage: "Server port",
			Value: 31234,
		},
		&cli.StringFlag{
			Name:  "host",
			Usage: "IP to listen on",
			Value: "localhost",
		},
	}
	app.Action = func(ctx *cli.Context) error {
		conf := api.Config{
			Host:        ctx.String("host"),
			Port:        ctx.Int("port"),
			Certificate: ctx.String("cert"),
			Key:         ctx.String("key"),
			CA:          ctx.String("ca"),
		}

		if err := api.Serve(conf); err != nil {
			return fmt.Errorf("error starting grpc server: %v", err)
		}
		return nil
	}
	app.Commands = []*cli.Command{
		{
			// re-execute a command, for the sake of avoiding cgroup race conditions
			Name: "rexec",
			Action: func(c *cli.Context) error {
				if err := worker.Rexec(c.Args().First(), c.Args().Tail()); err != nil {
					log.Fatalf("failed re-execing job: %v", err)
				}
				return nil
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
