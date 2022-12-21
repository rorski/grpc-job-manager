package worker

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Output takes a context and UUID and returns a channel of data from the output file
// A gRPC server can then read bytes off of the data stream to send to the client.
func (w *Worker) Output(ctx context.Context, uuid string) (chan []byte, error) {
	job, err := w.getJobByUUID(uuid)
	if err != nil {
		return nil, err
	}

	// path to the output file (e.g., /tmp/jobmanager/d8eb044d-073e-425d-928e-1e012975e451)
	outFilePath := filepath.Join(w.Config.Outpath, uuid)
	f, err := os.Open(outFilePath)
	if err != nil {
		return nil, err
	}
	dataStream := make(chan []byte)
	// stream data from the output file, passing in the job to check its status
	go func(job *Job) {
		// close the file and dataStream after streaming
		defer func() {
			if err := f.Close(); err != nil {
				log.Printf("error closing the output file: %v", err)
			}
			close(dataStream)
		}()

		// listen for filesystem events from the eventStream and read data to the
		// dataStream if the event is an IN_MODIFY (i.e., a write to the output file)
		eventStream, err := watch(ctx, outFilePath)
		if err != nil {
			log.Printf("error watching for file events: %v", err)
			return
		}
		if err := w.readChunk(ctx, f, dataStream); err != nil {
			if err == io.EOF {
				// if we're at the end of a file and the process is finished, exit the stream
				w.mu.RLock()
				isExited := job.status.Exited
				w.mu.RUnlock()
				if isExited {
					return
				}
			} else {
				log.Printf("error reading output file: %v", err)
				return
			}
		}
		for {
			if err := waitForModifyEvent(ctx, eventStream); err != nil {
				log.Printf("error waiting for IN_MODIFY event: %v", err)
				return
			}
			if err := w.readChunk(ctx, f, dataStream); err != nil {
				if err != io.EOF {
					log.Printf("error reading from output file %s: %v", f.Name(), err)
					return
				}
			}
		}
	}(job)

	return dataStream, nil
}

// Watch watches a file for IN_MODIFY events when it is written to.
// Note that this will not catch if the file is closed/moved because we are not
// watching for those events.
//
// See:
// https://linux.die.net/man/1/inotifywait
// https://pkg.go.dev/github.com/fsnotify/fsnotify
// https://efreitasn.dev/posts/inotify-api/
func watch(ctx context.Context, outFilePath string) (chan uint32, error) {
	fd, err := unix.InotifyInit()
	if err != nil {
		return nil, err
	}
	// add inotifywatch for IN_MODIFY events on a file
	wd, err := unix.InotifyAddWatch(fd, outFilePath, unix.IN_MODIFY)
	if err != nil {
		if err := unix.Close(fd); err != nil {
			log.Printf("error closing file descriptor: %v", err)
		}
		return nil, err
	}

	// channel for parsing inotify Masks - https://pkg.go.dev/golang.org/x/sys/unix#InotifyEvent
	eventStream := make(chan uint32)
	go func() {
		defer func() {
			// remove the watch when we're done
			success, err := unix.InotifyRmWatch(fd, uint32(wd))
			if success == -1 || err != nil {
				log.Printf("error removing inotify watch: %v", err)
			}
			if err := unix.Close(fd); err != nil {
				log.Printf("error closing file descriptor: %v", err)
			}
			close(eventStream)
		}()

		// read events from the fd
		// see "Reading Events" from https://efreitasn.dev/posts/inotify-api/
		var buf [(unix.SizeofInotifyEvent + unix.NAME_MAX + 1) * 20]byte
		for {
			n, err := unix.Read(fd, buf[:])
			if err != nil {
				log.Printf("error reading from fd: %v", err)
				return
			}
			offset := 0
			for offset <= n-unix.SizeofInotifyEvent {
				rawEvent := (*unix.InotifyEvent)(unsafe.Pointer(&buf[offset]))
				offset += unix.SizeofInotifyEvent + int(rawEvent.Len)
				// if this is not an IN_MODIFY event, continue to next "for" iteration
				if rawEvent.Mask&unix.IN_MODIFY != unix.IN_MODIFY {
					continue
				}
				// otherwise, send it to the eventStream
				select {
				case eventStream <- rawEvent.Mask:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return eventStream, nil
}

// waitForModifyEvent waits for IN_MODIFY events on the eventStream channel
func waitForModifyEvent(ctx context.Context, eventStream chan uint32) error {
	for {
		select {
		case event, ok := <-eventStream:
			if !ok {
				log.Print("eventStream channel closed")
			}
			if event&unix.IN_MODIFY == unix.IN_MODIFY {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// read chunks from file and send them to dataStream (by default, 64KB)
func (w *Worker) readChunk(ctx context.Context, file *os.File, dataStream chan []byte) error {
	for {
		chunk := make([]byte, w.Config.ChunkSize)
		n, err := file.Read(chunk)
		if err != nil {
			if n > 0 {
				// send remaining bytes through the data channel before returning
				dataStream <- chunk[:n]
			}
			return err
		}
		select {
		case dataStream <- chunk[:n]: // send the number of bytes read above through dataStream
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
