package worker

import (
	"fmt"
	"syscall"
)

// Stop terminates a running process
func (w *Worker) Stop(uuid string) error {
	job, err := w.getJobByUUID(uuid)
	if err != nil {
		return fmt.Errorf("error getting job: %v", err)
	}

	if err = job.cmd.Process.Signal(syscall.SIGKILL); err != nil {
		return fmt.Errorf("error killing process: %v", err)
	}
	w.mu.Lock()
	job.status.Terminated = true
	w.mu.Unlock()

	return nil
}
