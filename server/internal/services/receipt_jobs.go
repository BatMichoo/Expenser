package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"expenser/internal/models"

	"github.com/google/uuid"
)

// Reading a receipt can take Gemini minutes when it's busy, longer than a
// request should wait (nginx gives up first). Uploads therefore start a job
// that runs in the background; the page checks back until it's done.
const (
	receiptJobTimeout = 5 * time.Minute  // how long one read may take
	receiptJobTTL     = 30 * time.Minute // how long a job is kept
)

// JobStatus is where a receipt job is.
type JobStatus string

const (
	JobReading JobStatus = "reading"
	JobDone    JobStatus = "done"
	JobFailed  JobStatus = "failed"
)

// ReceiptJob is one receipt being read. Get returns copies.
type ReceiptJob struct {
	ID      string
	UserID  uuid.UUID
	Status  JobStatus
	Result  *models.ReceiptAnalysis
	Err     error
	Started time.Time
	seq     uint64 // start order, for Latest
}

// ReceiptJobs runs receipt reads in the background and keeps their results
// in memory for a while (a restart forgets them).
type ReceiptJobs struct {
	analyze func(ctx context.Context, data []byte) (*models.ReceiptAnalysis, error)
	now     func() time.Time

	mu   sync.Mutex
	jobs map[string]*ReceiptJob
	seq  uint64
}

func NewReceiptJobs(analyze func(ctx context.Context, data []byte) (*models.ReceiptAnalysis, error)) *ReceiptJobs {
	return &ReceiptJobs{analyze: analyze, now: time.Now, jobs: map[string]*ReceiptJob{}}
}

func newJobID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("receipt job id: %v", err))
	}
	return hex.EncodeToString(b)
}

// Start begins reading data for the user and returns the job's ID at once.
func (j *ReceiptJobs) Start(userID uuid.UUID, data []byte) string {
	job := &ReceiptJob{ID: newJobID(), UserID: userID, Status: JobReading, Started: j.now()}
	j.mu.Lock()
	j.pruneLocked()
	j.seq++
	job.seq = j.seq
	j.jobs[job.ID] = job
	j.mu.Unlock()

	go func() {
		// Not the request's context: the upload request returns right away.
		ctx, cancel := context.WithTimeout(context.Background(), receiptJobTimeout)
		defer cancel()
		result, err := j.analyze(ctx, data)

		j.mu.Lock()
		defer j.mu.Unlock()
		if err != nil {
			log.Printf("receipt job %s: %v", job.ID, err)
			job.Status, job.Err = JobFailed, err
			return
		}
		job.Status, job.Result = JobDone, result
	}()
	return job.ID
}

// Get returns the user's job, if it exists.
func (j *ReceiptJobs) Get(id string, userID uuid.UUID) (ReceiptJob, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.pruneLocked()
	job, ok := j.jobs[id]
	if !ok || job.UserID != userID {
		return ReceiptJob{}, false
	}
	return *job, true
}

// Latest returns the user's most recent job, so reopening the upload panel
// shows a read that is still going or finished while it was closed.
func (j *ReceiptJobs) Latest(userID uuid.UUID) (ReceiptJob, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.pruneLocked()
	var latest *ReceiptJob
	for _, job := range j.jobs {
		if job.UserID == userID && (latest == nil || job.seq > latest.seq) {
			latest = job
		}
	}
	if latest == nil {
		return ReceiptJob{}, false
	}
	return *latest, true
}

// Forget drops a job once its result has been shown (or the user starts over).
func (j *ReceiptJobs) Forget(id string, userID uuid.UUID) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if job, ok := j.jobs[id]; ok && job.UserID == userID {
		delete(j.jobs, id)
	}
}

func (j *ReceiptJobs) pruneLocked() {
	cutoff := j.now().Add(-receiptJobTTL)
	for id, job := range j.jobs {
		if job.Started.Before(cutoff) {
			delete(j.jobs, id)
		}
	}
}
