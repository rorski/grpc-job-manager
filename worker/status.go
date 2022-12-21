package worker

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Status returns the current status of a process
func (w *Worker) Status(uuid string) (status Status, err error) {
	job, err := w.getJobByUUID(uuid)
	if err != nil {
		return Status{}, err
	}
	// get exited boolean and exitcode with a read lock
	w.mu.RLock()
	exited, exitCode := job.status.Exited, job.status.ExitCode
	w.mu.RUnlock()

	var processStat ProcessStat
	// only try to grab the job status from /proc/<pid>/stat if the job hasn't exited
	if !exited && exitCode == 0 {
		processStat, err = parseProcStat(strconv.Itoa(job.pid))
		if err != nil {
			return Status{}, err
		}
		switch processStat.State {
		case "R", "S", "D":
			processStat.State = "RUNNING"
		case "Z":
			processStat.State = "ZOMBIE"
		case "T":
			processStat.State = "STOPPED"
		}
	} else {
		processStat.State = "EXITED"
	}
	w.mu.Lock()
	job.status.State = processStat.State
	w.mu.Unlock()

	return *job.status, nil
}

// parse the /proc/<pid>/stat file to get information about a process. This is used
// to get the process state for getProcessState() and the PID for Rexec()
// Note that pid here is a string because it could be "self"
// See: /proc/[pid]/stat section of https://man7.org/linux/man-pages/man5/proc.5.html
func parseProcStat(pid string) (stat ProcessStat, err error) {
	stats, err := os.ReadFile(filepath.Join("/", "proc", pid, "stat"))
	if err != nil {
		return ProcessStat{}, fmt.Errorf("error reading /proc/%s/stat: %v", pid, err)
	}

	var ignoreString string
	if _, err := fmt.Fscan(bytes.NewBuffer(stats), &stat.PID, &ignoreString, &stat.State); err != nil {
		return ProcessStat{}, err
	}

	return stat, err
}
