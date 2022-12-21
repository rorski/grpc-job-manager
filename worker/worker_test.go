package worker

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

var worker = New()

func TestStartJob(t *testing.T) {
	UUID, err := worker.Start("ps", []string{})
	assert.Nil(t, err)
	assert.NotEmpty(t, UUID)
}

func TestStopJob(t *testing.T) {
	UUID, err := worker.Start("top", []string{})
	assert.NoError(t, err)

	time.Sleep(time.Second)
	err = worker.Stop(UUID)
	assert.NoError(t, err)
}

func TestStopBadJob(t *testing.T) {
	err := worker.Stop(uuid.NewString())
	assert.NotNil(t, err)
}

func TestJobStatusRunning(t *testing.T) {
	UUID, err := worker.Start("top", []string{})
	assert.NoError(t, err)

	time.Sleep(time.Second)
	status, err := worker.Status(UUID)
	assert.NoError(t, err)
	assert.Equal(t, status.State, "RUNNING")
	assert.Equal(t, false, status.Terminated)

	err = worker.Stop(UUID)
	assert.NoError(t, err)
}

func TestJobStatusStopped(t *testing.T) {
	UUID, err := worker.Start("top", []string{})
	assert.NoError(t, err)

	time.Sleep(time.Second)
	err = worker.Stop(UUID)
	assert.NoError(t, err)

	time.Sleep(time.Second)
	status, err := worker.Status(UUID)
	assert.NoError(t, err)
	assert.Equal(t, status.State, "EXITED")
	assert.Equal(t, true, status.Terminated)
}

func TestJobStatusBad(t *testing.T) {
	status, err := worker.Status(uuid.NewString())
	assert.Error(t, err)
	assert.Equal(t, Status{}, status)
}

// TestOutputJob creates a file under /tmp/jobmanager with 512 bytes of random data using rand.Read().
// The test function generates a hash of that data and compares it to the output from the Output()
// method to ensure they match.
func TestOutputJob(t *testing.T) {
	randomData := make([]byte, 512)
	_, err := rand.Read(randomData)
	assert.NoError(t, err)
	// generate sha256 hash of random data
	firstHash := sha256.Sum256(randomData)

	// create a UUID and dummy job so output finds an exited job to parse
	UUID := uuid.NewString()
	worker.jobs[UUID] = &Job{UUID: UUID, status: &Status{Exited: true}}

	// create the output file
	f, err := createOutFile(UUID)
	assert.NoError(t, err)
	defer f.Close()
	// write the random data to the output file
	_, err = f.Write(randomData)
	assert.NoError(t, err)

	// read output file through Output() method
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	dataStream, err := worker.Output(ctx, UUID)
	assert.NoError(t, err)
	assert.NotNil(t, dataStream)

	// hash the data read through the data stream
	secondHash := sha256.Sum256(<-dataStream)
	// assert that the original data matches the data read through Output()
	assert.EqualValues(t, firstHash, secondHash)
}

func TestOutputJobBad(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	dataStream, err := worker.Output(ctx, uuid.NewString())
	assert.Nil(t, dataStream)
	assert.Error(t, err)
}
