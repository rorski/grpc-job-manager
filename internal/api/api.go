package api

import (
	"context"
	"fmt"
	"log"

	"github.com/rorski/grpc-job-manager/internal/job"
	"github.com/rorski/grpc-job-manager/worker"
)

type jobManagerServer struct {
	job.UnimplementedJobManagerServer
	Worker worker.Worker
}

// Start takes a linux command with arguments to run on the worker.
// If successful, it returns the UUID, which can be used to reference the job for other methods (stop, status, and output).
//
// Roles: [admin]
func (s *jobManagerServer) Start(c context.Context, in *job.StartRequest) (*job.StartResponse, error) {
	res, err := s.Worker.Start(in.GetCmd(), in.GetArgs())
	if err != nil {
		return nil, fmt.Errorf("error starting job: %v", err)
	}
	return &job.StartResponse{Uuid: res}, nil
}

// Stop takes a UUID and stops the job, if it is still running.
//
// Roles: [admin]
func (s *jobManagerServer) Stop(c context.Context, in *job.StopRequest) (*job.StopResponse, error) {
	if err := s.Worker.Stop(in.GetUuid()); err != nil {
		return nil, err
	}
	return &job.StopResponse{}, nil
}

// Status takes a UUID and gets the status of the job
// If successful, it returns the state of the job (RUNNING, STOPPED, ZOMBIE) or EXITED if the job is done
//
// Roles: [admin, user]
func (s *jobManagerServer) Status(c context.Context, in *job.StatusRequest) (*job.StatusResponse, error) {
	res, err := s.Worker.Status(in.GetUuid())
	if err != nil {
		return nil, fmt.Errorf("error getting process status: %v", err)
	}
	return &job.StatusResponse{Status: res.State, Terminated: res.Terminated, ExitCode: int32(res.ExitCode)}, nil
}

// Output takes a UUID and streams the output of the job through a dataStream channel
//
// Roles: [admin, user]
func (s *jobManagerServer) Output(in *job.OutputRequest, stream job.JobManager_OutputServer) error {
	dataStream, err := s.Worker.Output(stream.Context(), in.GetUuid())
	if err != nil {
		return fmt.Errorf("error getting data stream: %v", err)
	}
	for {
		select {
		// if the context is cancelled, close the channel
		case <-stream.Context().Done():
			log.Print("stream context cancelled")
			return stream.Context().Err()
		// read data off the stream (up to the chunk size set in the Worker library)
		case data, ok := <-dataStream:
			if !ok {
				return nil
			}
			if err := stream.Send(&job.OutputResponse{Output: data}); err != nil {
				return fmt.Errorf("error sending data from stream: %v", err)
			}
		}
	}
}
