package worker

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/google/uuid"
)

const cgroupPath = "/sys/fs/cgroup" // path to the top level cgroup v1 hierarchy

// map of cgroup controllers to configured parameter files
// these are hard coded but in production they would be configurable
var cgroupParamsMap = map[string]map[string]string{
	"blkio": {
		"blkio.bfq.weight": "500",
	},
	"cpu,cpuacct": {
		"cpu.shares": "128",
	},
	"memory": {
		"memory.limit_in_bytes": "32M",
	},
}

// Start creates a new process
func (w *Worker) Start(name string, args []string) (string, error) {
	// create a unique ID to identify the process, since a process ID could be reused
	uniqueJobId := uuid.NewString()
	outfile, err := createOutFile(uniqueJobId)
	if err != nil {
		if closeErr := outfile.Close(); err != nil {
			log.Printf("error closing output file: %v", closeErr)
		}
		return "", fmt.Errorf("error creating temp file: %v", err)
	}

	// pass in /proc/self/exe so we re-execute this process in an isolated namespace with cgroup restrictions
	cmd := exec.Command("/proc/self/exe", append([]string{"rexec", name}, args...)...)
	cmd.Stdout = outfile
	cmd.Stderr = outfile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// create an isolated pid and mount namespace
		Cloneflags:   syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		Unshareflags: syscall.CLONE_NEWNS,
		Pdeathsig:    syscall.SIGTERM, // terminate the child process if this parent dies
	}
	log.Printf("created job: %s\n", uniqueJobId)
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("error running command: %v", err)
	}

	// create new Job with the details of this job and add to the Jobs map
	job := &Job{
		UUID: uniqueJobId,
		cmd:  cmd,
		pid:  cmd.Process.Pid,
		status: &Status{
			Terminated: false,
		},
	}
	w.mu.Lock()
	w.jobs[uniqueJobId] = job
	w.mu.Unlock()

	// wait for process to complete in the background
	go func() {
		if err = cmd.Wait(); err != nil {
			log.Printf("job finished with error: %v\n", err)
		}
		log.Printf("job finished at pid: %d\n", cmd.Process.Pid)
		w.mu.Lock()
		// update the status with the exit code of the process
		job.status.ExitCode = job.cmd.ProcessState.ExitCode()
		job.status.Exited = job.cmd.ProcessState.Exited()
		w.mu.Unlock()

		// clean up cgroups after the job completes
		if err = removeCgroups(cmd.Process.Pid); err != nil {
			log.Printf("error removing cgroup directories for %d: %v\n", cmd.Process.Pid, err)
		}
		if err = outfile.Close(); err != nil {
			log.Printf("error closing output file %s: %v", outfile.Name(), err)
		}
	}()

	return job.UUID, nil
}

// Rexec re-executes a command and places it in the same cgroup as its parent
func Rexec(name string, args []string) error {
	// Get the parent process (/proc/self/exe rexec ...) PID to use for creating a cgroup of the same name
	processState, err := parseProcStat("self")
	if err != nil {
		return err
	}
	if err := createCgroup(processState.PID); err != nil {
		return fmt.Errorf("error adding job to cgroup: %v", err)
	}

	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// terminate the child process if this parent dies
		Pdeathsig: syscall.SIGKILL,
	}

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

// create the output file for a job. If the jobmanager directory (/tmp/jobmanager) doesn't exist, create it.
func createOutFile(uuid string) (*os.File, error) {
	jobsDir := filepath.Join(os.TempDir(), "jobmanager") // this should be configured somewhere
	// make sure the jobmanager output directory exists
	if _, err := os.Stat(jobsDir); err != nil {
		if os.IsNotExist(err) {
			log.Print("creating job output directory")
			if err = os.Mkdir(jobsDir, 0644); err != nil {
				return nil, fmt.Errorf("could not create directory %s", jobsDir)
			}
		} else {
			return nil, fmt.Errorf("error getting fileinfo on %s: %v", jobsDir, err)
		}
	}

	return os.OpenFile(filepath.Join(jobsDir, uuid), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
}

// given a passed in path like "/sys/fs/cgroup/blkio/12345", create the correct
// params file under that cgroup and add the process to cgroup.procs
func configureCgroup(path string, params map[string]string) error {
	// for every defined parameter in the controller, write that file with the
	// appropriate setting from the cgroupParamsMap above
	for param := range params {
		paramsFile, err := os.OpenFile(filepath.Join(path, param), os.O_APPEND|os.O_WRONLY, 0555)
		if err != nil {
			return fmt.Errorf("error creating cgroup parameters file: %v", err)
		}
		if _, err = paramsFile.WriteString(params[param] + "\n"); err != nil {
			return fmt.Errorf("error writing process to cgroup: %v", err)
		}
		if err = paramsFile.Close(); err != nil {
			return fmt.Errorf("error closing cgroup parameters file %s: %v", paramsFile.Name(), err)
		}
	}

	// write the process id to the cgroup.procs in this cgroup. Note the pid written
	// will match the path, since we're doing a cgroup-per-process model
	procsFile, err := os.OpenFile(filepath.Join(path, "cgroup.procs"), os.O_APPEND|os.O_WRONLY, 0555)
	if err != nil {
		return fmt.Errorf("error creating cgroup.procs file: %v", err)
	}
	defer procsFile.Close()
	// writing "0" to a cgroup causes the writing process to be moved to that cgroup.
	// see "Creating cgroups and moving processes": https://man7.org/linux/man-pages/man7/cgroups.7.html
	if _, err = procsFile.WriteString(strconv.Itoa(0)); err != nil {
		return fmt.Errorf("error writing process to cgroup: %v", err)
	}

	return nil
}

// create a new cgroup in each of the three controllers: blkio, cpu, and memory
// 1. Create <pid> under each of the three cgroups
// 2. add a cgroups.proc file and the relevant parameter file to each cgroup
func createCgroup(pid string) error {
	for controller, params := range cgroupParamsMap {
		cgroupPidPath := filepath.Join(cgroupPath, controller, pid)
		if err := os.Mkdir(cgroupPidPath, 0555); err != nil {
			return fmt.Errorf("error creating %s: %v", cgroupPidPath, err)
		}
		if err := configureCgroup(cgroupPidPath, params); err != nil {
			return err
		}
	}
	return nil
}

// clean up (remove) the cgroup once the job is finished
func removeCgroups(pid int) error {
	var errorStrings []string
	for controller := range cgroupParamsMap {
		// path to the cgroup for this process
		cgroupPidPath := filepath.Join(cgroupPath, controller, strconv.Itoa(pid))
		if err := os.RemoveAll(cgroupPidPath); err != nil {
			errorStrings = append(errorStrings, fmt.Sprintf("error removing %s: %v", cgroupPidPath, err.Error()))
		}
	}
	if len(errorStrings) != 0 {
		return errors.New(strings.Join(errorStrings, " "))
	}
	return nil
}
