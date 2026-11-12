package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"expenser/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitFor polls until the job leaves "reading" (jobs finish in goroutines).
func waitFor(t *testing.T, j *ReceiptJobs, id string, user uuid.UUID) ReceiptJob {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := j.Get(id, user)
		require.True(t, ok)
		if job.Status != JobReading {
			return job
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return ReceiptJob{}
}

func TestReceiptJobReadsInTheBackground(t *testing.T) {
	release := make(chan struct{})
	var gotData []byte
	jobs := NewReceiptJobs(func(ctx context.Context, data []byte) (*models.ReceiptAnalysis, error) {
		gotData = data
		<-release
		_, hasDeadline := ctx.Deadline()
		assert.True(t, hasDeadline, "each job has a time limit")
		return &models.ReceiptAnalysis{Store: "Kaufland"}, nil
	})
	user := uuid.New()
	id := jobs.Start(user, []byte("pdf"))

	job, ok := jobs.Get(id, user)
	require.True(t, ok)
	assert.Equal(t, JobReading, job.Status, "the upload returns before Gemini answers")

	close(release)
	job = waitFor(t, jobs, id, user)
	assert.Equal(t, JobDone, job.Status)
	assert.Equal(t, "Kaufland", job.Result.Store)
	assert.Equal(t, []byte("pdf"), gotData)
}

func TestReceiptJobFailure(t *testing.T) {
	boom := errors.New("gemini says no")
	jobs := NewReceiptJobs(func(context.Context, []byte) (*models.ReceiptAnalysis, error) { return nil, boom })
	user := uuid.New()
	id := jobs.Start(user, nil)
	job := waitFor(t, jobs, id, user)
	assert.Equal(t, JobFailed, job.Status)
	assert.ErrorIs(t, job.Err, boom)
}

func TestReceiptJobsAreTheirOwners(t *testing.T) {
	jobs := NewReceiptJobs(func(context.Context, []byte) (*models.ReceiptAnalysis, error) { return &models.ReceiptAnalysis{}, nil })
	owner := uuid.New()
	id := jobs.Start(owner, nil)
	_, ok := jobs.Get(id, uuid.New())
	assert.False(t, ok, "someone else's job is not found")
	_, ok = jobs.Get("nope", owner)
	assert.False(t, ok)
}

func TestReceiptJobsLatestAndForget(t *testing.T) {
	jobs := NewReceiptJobs(func(context.Context, []byte) (*models.ReceiptAnalysis, error) { return &models.ReceiptAnalysis{}, nil })
	user := uuid.New()
	_, ok := jobs.Latest(user)
	assert.False(t, ok)

	first := jobs.Start(user, nil)
	second := jobs.Start(user, nil)
	latest, ok := jobs.Latest(user)
	require.True(t, ok)
	assert.Equal(t, second, latest.ID, "the newest upload")

	jobs.Forget(second, user)
	latest, ok = jobs.Latest(user)
	require.True(t, ok)
	assert.Equal(t, first, latest.ID)
	jobs.Forget(first, user)
	_, ok = jobs.Latest(user)
	assert.False(t, ok)
}

func TestReceiptJobsExpire(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	jobs := NewReceiptJobs(func(context.Context, []byte) (*models.ReceiptAnalysis, error) { return &models.ReceiptAnalysis{}, nil })
	jobs.now = func() time.Time { return now }
	user := uuid.New()
	id := jobs.Start(user, nil)
	waitFor(t, jobs, id, user)

	now = now.Add(receiptJobTTL + time.Minute)
	_, ok := jobs.Get(id, user)
	assert.False(t, ok, "old jobs are dropped")
}
