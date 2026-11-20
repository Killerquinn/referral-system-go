// Package queue runs background jobs with River (a job queue stored in Postgres, see migration 000007).
//
// Jobs:
//   - process_upload   — enqueued by UploadService.CompleteUpload, runs UploadService.ProcessUpload
//   - cleanup_uploads  — periodic (every 10m + on start), runs UploadService.CleanupStale
//   - cleanup_page_views — periodic (every 6h + on start), runs PageService.CleanupViewVisitors
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

// ViewsCleaner drops old page-view dedupe rows. Implemented by *pageservice.PageService.
type ViewsCleaner interface {
	CleanupViewVisitors(ctx context.Context) (int, error)
}

type ExpiredSessions interface {
	CleanupExpiredSessions(ctx context.Context) (int, error)
}

const (
	viewsCleanupInterval           = 6 * time.Hour
	expiredSessionsCleanupInterval = 12 * time.Hour
	processUploadMaxAttempts       = 5
	processUploadTimeout           = 3 * time.Minute
	cleanupInterval                = 10 * time.Minute
	cleanupTimeout                 = 2 * time.Minute
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

// ===== cleanup_page_views =====

type CleanupPageViewsArgs struct{}

func (CleanupPageViewsArgs) Kind() string { return "cleanup_page_views" }

type cleanupPageViewsWorker struct {
	river.WorkerDefaults[CleanupPageViewsArgs]
	q *Queue
}

func (w *cleanupPageViewsWorker) Work(ctx context.Context, _ *river.Job[CleanupPageViewsArgs]) error {
	n, err := w.q.views.CleanupViewVisitors(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		w.q.log.Info("page view cleanup done", zap.Int("deleted", n))
	}
	return nil
}

// ===== cleanup_expired_sessions =====

type CleanupExpiredSessionsArgs struct{}

func (CleanupExpiredSessionsArgs) Kind() string { return "cleanup_expired_sessions" }

type cleanupExpiredSessionsWorker struct {
	river.WorkerDefaults[CleanupExpiredSessionsArgs]
	q *Queue
}

func (w *cleanupExpiredSessionsWorker) Work(ctx context.Context, _ *river.Job[CleanupExpiredSessionsArgs]) error {
	n, err := w.q.sessions.CleanupExpiredSessions(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		w.q.log.Info("expired sessions cleanup done", zap.Int("deleted", n))
	}
	return nil
}

// ===== client =====

type Queue struct {
	client    *river.Client[pgx.Tx]
	processor UploadProcessor
	views     ViewsCleaner
	sessions  ExpiredSessions
	log       *zap.Logger
}

// New builds the client. Workers need the upload service and the service needs the queue
// (to enqueue), so the processor is attached afterwards with SetUploadProcessor, before Start.
func New(log *zap.Logger, pool *pgxpool.Pool) (*Queue, error) {
	q := &Queue{log: log.With(zap.String("component", "queue"))}

	workers := river.NewWorkers()
	river.AddWorker(workers, &processUploadWorker{q: q})
	river.AddWorker(workers, &cleanupUploadsWorker{q: q, log: q.log})
	river.AddWorker(workers, &cleanupPageViewsWorker{q: q})
	river.AddWorker(workers, &cleanupExpiredSessionsWorker{q: q})

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: maxWorkers}},
		Workers: workers,
		PeriodicJobs: []*river.PeriodicJob{
			river.NewPeriodicJob(
				river.PeriodicInterval(cleanupInterval),
				func() (river.JobArgs, *river.InsertOpts) { return CleanupUploadsArgs{}, nil },
				&river.PeriodicJobOpts{RunOnStart: true},
			),
			river.NewPeriodicJob(
				river.PeriodicInterval(viewsCleanupInterval),
				func() (river.JobArgs, *river.InsertOpts) { return CleanupPageViewsArgs{}, nil },
				&river.PeriodicJobOpts{RunOnStart: true},
			),
			river.NewPeriodicJob(
				river.PeriodicInterval(expiredSessionsCleanupInterval),
				func() (river.JobArgs, *river.InsertOpts) { return CleanupExpiredSessionsArgs{}, nil },
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

func (q *Queue) SetViewsCleaner(v ViewsCleaner) { q.views = v }

func (q *Queue) SetSessionsCleaner(ex ExpiredSessions) { q.sessions = ex }

// Start begins fetching and working jobs. Returns immediately.
func (q *Queue) Start(ctx context.Context) error {
	if q.processor == nil || q.views == nil || q.sessions == nil {
		return fmt.Errorf("queue: upload processor / views cleaner / session cleaner is not set")
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
