package processing

import (
	"context"
	"errors"
	"testing"
)

type jobRepository struct {
	job       Job
	available bool
	completed bool
	failed    bool
}

func (repository *jobRepository) ClaimProcessingJob(context.Context) (Job, bool, error) {
	if !repository.available {
		return Job{}, false, nil
	}
	repository.available = false
	return repository.job, true, nil
}
func (repository *jobRepository) CompleteProcessingJob(context.Context, Job, Metadata) error {
	repository.completed = true
	return nil
}
func (repository *jobRepository) FailProcessingJob(context.Context, Job, string) error {
	repository.failed = true
	return nil
}
func (*jobRepository) GetProcessingStatus(context.Context, string, string) (Job, error) {
	return Job{}, nil
}
func (*jobRepository) ListProcessingJobs(context.Context) ([]Job, error) { return nil, nil }
func (*jobRepository) RetryProcessingJob(context.Context, string) error  { return nil }

func TestProcessOnceCompletesOrRetriesJob(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "complete"},
		{name: "retry", err: errors.New("ffmpeg failed")},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &jobRepository{job: Job{ID: "job"}, available: true}
			service := New(repository, t.TempDir())
			service.process = func(context.Context, Job) (Metadata, error) { return Metadata{}, test.err }
			processed, err := service.ProcessOnce(context.Background())
			if !processed {
				t.Fatal("expected a claimed job")
			}
			if test.err == nil && (err != nil || !repository.completed || repository.failed) {
				t.Fatalf("completion state: err=%v complete=%t failed=%t", err, repository.completed, repository.failed)
			}
			if test.err != nil && (err == nil || repository.completed || !repository.failed) {
				t.Fatalf("retry state: err=%v complete=%t failed=%t", err, repository.completed, repository.failed)
			}
		})
	}
}
