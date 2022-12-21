package worker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

type Worker struct {
	mu     sync.RWMutex    // protects jobs map
	jobs   map[string]*Job // map of job UUID to Job
	Config *Config
}

type Config struct {
	ChunkSize int
	Outpath   string
}

// Job represents an arbitrary Linux process schedule by the Worker
type Job struct {
	UUID   string
	cmd    *exec.Cmd
	pid    int
	status *Status
}

// Status of the process
type Status struct {
	State      string // RUNNING, STOPPED, ZOMBIE, EXITED
	Terminated bool   // Job terminated by the worker API
	ExitCode   int    // https://pkg.go.dev/os#ProcessState.ExitCode
	Exited     bool   // https://pkg.go.dev/os#ProcessState.Exited
}

type ProcessStat struct {
	PID   string
	State string
}

func New() *Worker {
	return &Worker{
		jobs: make(map[string]*Job),
		Config: &Config{
			ChunkSize: 1024 * 64,                                 // set default chunk size to 64KB
			Outpath:   filepath.Join(os.TempDir(), "jobmanager"), // path to the output files, e.g., /tmp/jobmanager
		},
	}
}

func (w *Worker) getJobByUUID(uuid string) (*Job, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	job, ok := w.jobs[uuid]
	if !ok {
		return nil, fmt.Errorf("no job with uuid %s found", uuid)
	}
	return job, nil
}
