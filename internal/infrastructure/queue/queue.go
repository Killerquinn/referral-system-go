// Package queue runs background jobs with River (a job queue stored in Postgres, see migration 000007).
//
// Jobs:
//   - process_upload   — enqueued by UploadService.CompleteUpload, runs UploadService.ProcessUpload
//   - cleanup_uploads  — periodic (every 10m + on start), runs UploadService.CleanupStale
//
// Retries: a job whose Work returns an error is retried with backoff up to MaxAttempts,
// then it is discarded. Return nil for permanent failures you already handled (e.g. a broken file
// marked `failed`) so River doesn't retry them.
package queue

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"go.uber.org/zap"
)

// UploadProcessor is what the jobs call. Implemented by *uploadservice.UploadService.
type UploadProcessor interface {
	// attempt / maxAttempts let the service mark the upload `failed` on the last try.
	ProcessUpload(ctx context.Context, uploadID string, attempt, maxAttempts int) error
	CleanupStale(ctx context.Context) (int, error)
}

const (
	processUploadMaxAttempts = 5
	processUploadTimeout     = 3 * time.Minute
	cleanupInterval          = 10 * time.Minute
	cleanupTimeout           = 2 * time.Minute
	// libvips is CPU and memory heavy: a few at a time is plenty for now
	maxWorkers = 2
)

// ===== process_upload =====

type ProcessUploadArgs struct {
	UploadID string `json:"upload_id"`
}

func (ProcessUploadArgs) Kind() string { return "process_upload" }

func (ProcessUploadArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: processUploadMaxAttempts,
		// a second "complete" or a re-enqueue never creates a duplicate job for the same upload
		UniqueOpts: river.UniqueOpts{ByArgs: true},
	}
}

type processUploadWorker struct {
	river.WorkerDefaults[ProcessUploadArgs]
	q *Queue
}

func (w *processUploadWorker) Timeout(*river.Job[ProcessUploadArgs]) time.Duration {
	return processUploadTimeout
}

func (w *processUploadWorker) Work(ctx context.Context, job *river.Job[ProcessUploadArgs]) error {
	return w.q.processor.ProcessUpload(ctx, job.Args.UploadID, job.Attempt, job.MaxAttempts)
}

// ===== cleanup_uploads =====

type CleanupUploadsArgs struct{}

func (CleanupUploadsArgs) Kind() string { return "cleanup_uploads" }

type cleanupUploadsWorker struct {
	river.WorkerDefaults[CleanupUploadsArgs]
	q   *Queue
	log *zap.Logger
}

func (w *cleanupUploadsWorker) Timeout(*river.Job[CleanupUploadsArgs]) time.Duration {
	return cleanupTimeout
}

func (w *cleanupUploadsWorker) Work(ctx context.Context, _ *river.Job[CleanupUploadsArgs]) error {
	n, err := w.q.processor.CleanupStale(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		w.log.Info("upload cleanup done", zap.Int("deleted", n))
	}
	return nil
}

// ===== client =====

type Queue struct {
	client    *river.Client[pgx.Tx]
	processor UploadProcessor
	log       *zap.Logger
}

// New builds the client. Workers need the upload service and the service needs the queue
// (to enqueue), so the processor is attached afterwards with SetUploadProcessor, before Start.
func New(log *zap.Logger, pool *pgxpool.Pool) (*Queue, error) {
	q := &Queue{log: log.With(zap.String("component", "queue"))}

	workers := river.NewWorkers()
	river.AddWorker(workers, &processUploadWorker{q: q})
	river.AddWorker(workers, &cleanupUploadsWorker{q: q, log: q.log})

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: maxWorkers}},
		Workers: workers,
		PeriodicJobs: []*river.PeriodicJob{
			river.NewPeriodicJob(
				river.PeriodicInterval(cleanupInterval),
				func() (river.JobArgs, *river.InsertOpts) { return CleanupUploadsArgs{}, nil },
				&river.PeriodicJobOpts{RunOnStart: true},
			),
		},
		// River logs via slog; only warnings and errors, our own logs go through zap
		Logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn})),
	})
	if err != nil {
		return nil, fmt.Errorf("queue: new client: %w", err)
	}
	q.client = client
	return q, nil
}

func (q *Queue) SetUploadProcessor(p UploadProcessor) { q.processor = p }

// Start begins fetching and working jobs. Returns immediately.
func (q *Queue) Start(ctx context.Context) error {
	if q.processor == nil {
		return fmt.Errorf("queue: upload processor is not set")
	}
	return q.client.Start(ctx)
}

// Stop waits for running jobs to finish (until ctx expires).
func (q *Queue) Stop(ctx context.Context) error {
	return q.client.Stop(ctx)
}

// EnqueueProcessUpload implements uploadservice.processingQueue.
func (q *Queue) EnqueueProcessUpload(ctx context.Context, uploadID string) error {
	if _, err := q.client.Insert(ctx, ProcessUploadArgs{UploadID: uploadID}, nil); err != nil {
		return fmt.Errorf("queue: enqueue process_upload: %w", err)
	}
	return nil
}
