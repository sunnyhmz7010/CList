package storage

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/sunnyhmz7010/CList/internal/jobs"
)

type PutResult struct {
	Object Object
	Job    jobs.Job
	Err    error
}

type putPayload struct {
	ProfileID string     `json:"profile_id"`
	Meta      ObjectMeta `json:"meta"`
}

type Orchestrator struct {
	registry *Registry
	jobs     *jobs.Store
}

func NewOrchestrator(registry *Registry, store *jobs.Store) *Orchestrator {
	return &Orchestrator{registry: registry, jobs: store}
}

func (o *Orchestrator) Put(ctx context.Context, profileID string, stream io.Reader, meta ObjectMeta) PutResult {
	job, err := o.jobs.Enqueue(ctx, "storage.put", putPayload{ProfileID: profileID, Meta: meta})
	if err != nil {
		return PutResult{Err: err}
	}
	if err := o.jobs.Begin(ctx, job.ID, time.Now().Add(30*time.Minute)); err != nil {
		return PutResult{Job: job, Err: err}
	}
	job, _ = o.jobs.Get(ctx, job.ID)
	backend, err := o.registry.Resolve(profileID)
	if err != nil {
		_ = o.jobs.Finish(ctx, job.ID, jobs.Failed, 0, nil, err)
		job, _ = o.jobs.Get(ctx, job.ID)
		return PutResult{Job: job, Err: err}
	}
	object, err := backend.Put(ctx, stream, meta)
	if err != nil {
		state := jobs.Failed
		resultErr := err
		if profileID != "local-default" && (errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)) {
			state = jobs.Uncertain
			resultErr = jobs.ErrUncertain
		}
		_ = o.jobs.Finish(ctx, job.ID, state, 0, nil, err)
		job, _ = o.jobs.Get(ctx, job.ID)
		return PutResult{Job: job, Err: resultErr}
	}
	_ = o.jobs.Finish(ctx, job.ID, jobs.Succeeded, 1, object, nil)
	job, _ = o.jobs.Get(ctx, job.ID)
	return PutResult{Object: object, Job: job}
}

func (o *Orchestrator) ResolveUncertain(ctx context.Context, jobID, action string, ref *TelegramRef) error {
	job, err := o.jobs.Get(ctx, jobID)
	if err != nil {
		return err
	}
	if job.State != jobs.Uncertain {
		return errors.New("任务不是不确定状态")
	}
	switch action {
	case "bind":
		if ref == nil || ref.ChatID == "" || ref.MessageID == 0 || ref.FileID == "" {
			return ErrInvalidKey
		}
		return o.jobs.Finish(ctx, jobID, jobs.Succeeded, 1, ref, nil)
	case "retry":
		return o.jobs.Requeue(ctx, jobID)
	case "fail":
		return o.jobs.Finish(ctx, jobID, jobs.Failed, job.Progress, nil, errors.New("管理员标记失败"))
	default:
		return errors.New("未知的不确定任务处理动作")
	}
}
