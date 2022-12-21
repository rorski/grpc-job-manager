package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/urfave/cli/v2"

	"github.com/rorski/grpc-job-manager/internal/job"
)

func validateUUID(u string) bool {
	if _, err := uuid.Parse(u); err != nil {
		return false
	}
	return true
}

func Start(jobClient job.JobManagerClient, c *cli.Context) error {
	ctx, cancel := context.WithTimeout(c.Context, 10*time.Second)
	defer cancel()

	res, err := jobClient.Start(ctx, &job.StartRequest{
		Cmd:  c.Args().First(),
		Args: c.Args().Tail(),
	})
	if err != nil {
		return err
	}
	fmt.Printf("Started job: %q\nUUID: %s\n", strings.Join(c.Args().Slice(), " "), res.Uuid)
	return nil
}

func Stop(jobClient job.JobManagerClient, c *cli.Context) error {
	uuid := c.Args().First()
	if !validateUUID(uuid) {
		return fmt.Errorf("could not parse uuid: %s", uuid)
	}

	ctx, cancel := context.WithTimeout(c.Context, 10*time.Second)
	defer cancel()

	if _, err := jobClient.Stop(ctx, &job.StopRequest{Uuid: uuid}); err != nil {
		return err
	}
	fmt.Printf("Stopped job: %s\n", uuid)
	return nil
}

func Status(jobClient job.JobManagerClient, c *cli.Context) error {
	uuid := c.Args().First()
	if !validateUUID(uuid) {
		return fmt.Errorf("could not parse uuid: %s", uuid)
	}

	ctx, cancel := context.WithTimeout(c.Context, 10*time.Second)
	defer cancel()

	res, err := jobClient.Status(ctx, &job.StatusRequest{Uuid: uuid})
	if err != nil {
		return err
	}
	fmt.Printf("Status of job: [%+v]\n", res)
	return nil
}

func Output(jobClient job.JobManagerClient, c *cli.Context) error {
	uuid := c.Args().First()
	if !validateUUID(uuid) {
		return fmt.Errorf("could not parse uuid: %s", uuid)
	}

	ctx, cancel := context.WithCancel(c.Context)
	defer cancel()

	stream, err := jobClient.Output(ctx, &job.OutputRequest{Uuid: uuid})
	if err != nil {
		log.Fatalf("Error streaming output: %v", err)
	}

	for {
		output, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("output stream failed: %v", err)
		}
		fmt.Printf("%s", output.GetOutput())
	}

	return nil
}
