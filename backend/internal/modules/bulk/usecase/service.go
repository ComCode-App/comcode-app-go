package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/config"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/activity"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/i18n"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jobscope"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/queue"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrForbidden      = errors.New("forbidden")
	ErrInvalidRequest = errors.New("invalid request")
)

type Notifier interface {
	Enqueue(ctx context.Context, in notifmodel.EnqueueInput) ([]notifmodel.Notification, error)
}

type Enqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

type ExecuteInput struct {
	Resource string
	Action   string
	Target   bulkengine.BulkTarget
	Locale   string
	// AccountID is the active account of an app route; required by account-scoped adapters
	// and rejected by platform ones.
	AccountID int64
	// Params are the action parameters of an account-scoped adapter.
	Params json.RawMessage
}

type ExecuteResult struct {
	Async   bool
	Job     BulkJobView
	Summary bulkengine.BulkSummary
	// Persisted is true when Job is set (async jobs and every account-scoped run).
	Persisted bool
}

// JobAccess limits which jobs a caller reaches. AccountID 0 means platform jobs only; a positive
// AccountID means that account's jobs only. AnyActor lets the caller reach other actors' jobs
// (company owners/admins); otherwise only the caller's own.
type JobAccess struct {
	AccountID int64
	AnyActor  bool
}

func (a JobAccess) allows(row db.BulkJob, actorID int64) bool {
	if a.AccountID == 0 {
		if row.AccountID.Valid {
			return false
		}
	} else if !row.AccountID.Valid || row.AccountID.Int64 != a.AccountID {
		return false
	}
	return a.AnyActor || row.ActorID == actorID
}

type BulkJobView struct {
	UUID          uuid.UUID              `json:"uuid"`
	Resource      string                 `json:"resource"`
	Action        string                 `json:"action"`
	Status        string                 `json:"status"`
	Target        bulkengine.BulkTarget  `json:"target,omitempty"`
	Params        json.RawMessage        `json:"params,omitempty"`
	Summary       bulkengine.BulkSummary `json:"summary,omitempty"`
	Error         *string                `json:"error,omitempty"`
	RollbackUntil *time.Time             `json:"rollback_until,omitempty"`
	AppliedAt     *time.Time             `json:"applied_at,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}

// Service orchestrates bulk jobs.
type Service struct {
	q        *db.Queries
	registry *bulkengine.Registry
	queue    Enqueuer
	notifier Notifier
	activity *activity.Recorder
	cfg      config.BulkConfig
	log      *slog.Logger
	syncMode bool
}

func New(
	q *db.Queries,
	reg *bulkengine.Registry,
	enq Enqueuer,
	notifier Notifier,
	rec *activity.Recorder,
	cfg config.BulkConfig,
	log *slog.Logger,
) *Service {
	if log == nil {
		log = slog.Default()
	}
	if cfg.SyncMax <= 0 {
		cfg.SyncMax = 50
	}
	if cfg.RollbackHours <= 0 {
		cfg.RollbackHours = 24
	}
	return &Service{
		q: q, registry: reg, queue: enq, notifier: notifier,
		activity: rec, cfg: cfg, log: log, syncMode: enq == nil,
	}
}

func (s *Service) Execute(ctx context.Context, actorID int64, in ExecuteInput) (ExecuteResult, error) {
	ctx = bulkengine.WithActor(ctx, actorID)
	def, adapter, err := s.registry.ActionDef(in.Resource, in.Action)
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	if in.Target.Scope != "ids" && in.Target.Scope != "query" {
		return ExecuteResult{}, fmt.Errorf("%w: target scope must be ids or query", ErrInvalidRequest)
	}
	scoped, isScoped := adapter.(bulkengine.AccountScoped)
	params := in.Params
	if len(params) == 0 {
		params = json.RawMessage("{}")
	}
	if isScoped {
		if in.AccountID <= 0 {
			return ExecuteResult{}, fmt.Errorf("%w: resource requires an account", ErrInvalidRequest)
		}
		ctx = jobscope.With(ctx, jobscope.Scope{AccountID: in.AccountID, ActorID: actorID, Params: params})
		if err := scoped.ValidateParams(ctx, in.Action, params); err != nil {
			return ExecuteResult{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
		}
	} else if in.AccountID > 0 {
		return ExecuteResult{}, fmt.Errorf("%w: resource is not account scoped", ErrInvalidRequest)
	}
	targets, err := adapter.ResolveTargets(ctx, in.Action, in.Target)
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	if len(targets) == 0 {
		return ExecuteResult{}, fmt.Errorf("%w: no targets resolved", ErrInvalidRequest)
	}
	locale := string(i18n.Normalize(in.Locale))
	targetJSON, _ := json.Marshal(in.Target)
	inline := len(targets) <= s.cfg.SyncMax
	if inline && !isScoped {
		summary, err := s.applyTargets(ctx, 0, adapter, def, in.Action, targets)
		if err != nil {
			return ExecuteResult{}, err
		}
		if s.activity != nil {
			uid := actorID
			s.activity.Record(ctx, &uid, "bulk.completed", in.Resource, nil, map[string]any{
				"action": in.Action, "total": summary.Total,
				"succeeded": summary.Succeeded, "failed": summary.Failed, "sync": true,
			}, nil)
		}
		return ExecuteResult{Async: false, Summary: summary}, nil
	}
	var accountID pgtype.Int8
	if isScoped {
		accountID = pgtype.Int8{Int64: in.AccountID, Valid: true}
	}
	row, err := s.q.CreateBulkJob(ctx, db.CreateBulkJobParams{
		Resource: in.Resource, Action: in.Action, ActorID: actorID,
		Locale: locale, Status: "queued", TargetJson: targetJSON,
		AccountID: accountID, ParamsJson: params,
	})
	if err != nil {
		return ExecuteResult{}, err
	}
	if s.activity != nil {
		uid := actorID
		s.activity.Record(ctx, &uid, "bulk.requested", in.Resource, &row.Uuid, map[string]any{
			"action": in.Action, "total": len(targets),
		}, nil)
	}
	if s.syncMode || (isScoped && inline) {
		// Account-scoped runs under the inline threshold are applied now, without a
		// notification; a failure is reported on the job, not as a request error.
		if err := s.runJob(ctx, row.ID, !inline); err != nil {
			s.log.Warn("bulk_inline_failed", "job", row.Uuid, "error", err)
		}
		updated, err := s.q.GetBulkJobByID(ctx, row.ID)
		if err != nil {
			return ExecuteResult{}, err
		}
		view := mapBulkJob(updated)
		return ExecuteResult{Async: !inline, Job: view, Summary: view.Summary, Persisted: true}, nil
	}
	task, err := queue.NewBulkProcessTask(row.ID)
	if err != nil {
		return ExecuteResult{}, err
	}
	if _, err := s.queue.Enqueue(queue.WithRequestID(ctx, task), asynq.Queue(queue.QueueBulk)); err != nil {
		return ExecuteResult{}, err
	}
	return ExecuteResult{Async: true, Job: mapBulkJob(row), Persisted: true}, nil
}

// ProcessBulk runs a queued job (worker entry point).
func (s *Service) ProcessBulk(ctx context.Context, jobID int64) error {
	return s.runJob(ctx, jobID, true)
}

func (s *Service) runJob(ctx context.Context, jobID int64, notify bool) error {
	job, err := s.q.MarkBulkJobProcessing(ctx, jobID)
	if err != nil {
		return err
	}
	ctx = bulkengine.WithActor(ctx, job.ActorID)
	if job.AccountID.Valid {
		ctx = jobscope.With(ctx, jobscope.Scope{
			AccountID: job.AccountID.Int64, ActorID: job.ActorID, Params: job.ParamsJson,
		})
	}
	def, adapter, err := s.registry.ActionDef(job.Resource, job.Action)
	if err != nil {
		return s.failBulk(ctx, jobID, err.Error())
	}
	var target bulkengine.BulkTarget
	if err := json.Unmarshal(job.TargetJson, &target); err != nil {
		return s.failBulk(ctx, jobID, err.Error())
	}
	targets, err := adapter.ResolveTargets(ctx, job.Action, target)
	if err != nil {
		return s.failBulk(ctx, jobID, err.Error())
	}
	summary, err := s.applyTargets(ctx, jobID, adapter, def, job.Action, targets)
	if err != nil {
		return s.failBulk(ctx, jobID, err.Error())
	}
	resultJSON, _ := json.Marshal(summary)
	var rollbackUntil pgtype.Timestamptz
	if def.Reversible && summary.Succeeded > 0 {
		rollbackUntil = pgtype.Timestamptz{
			Time:  time.Now().UTC().Add(time.Duration(s.cfg.RollbackHours) * time.Hour),
			Valid: true,
		}
	}
	completed, err := s.q.MarkBulkJobCompleted(ctx, db.MarkBulkJobCompletedParams{
		ID: jobID, ResultJson: resultJSON, RollbackUntil: rollbackUntil,
	})
	if err != nil {
		return err
	}
	if s.notifier != nil && notify {
		uid := job.ActorID
		loc := i18n.Normalize(job.Locale)
		channels := []string{notifmodel.ChannelInapp}
		actionURL := fmt.Sprintf("/platform/bulk/%s", completed.Uuid.String())
		if job.AccountID.Valid {
			actionURL = fmt.Sprintf("/account/bulk?job=%s", completed.Uuid.String())
		}
		_, _ = s.notifier.Enqueue(ctx, notifmodel.EnqueueInput{
			UserID: &uid, Channels: channels,
			TemplateCode: "bulk.completed", Language: job.Locale,
			TemplateVars: map[string]string{
				"resource":  i18n.ResourceLabel(loc, job.Resource),
				"action":    job.Action,
				"succeeded": fmt.Sprintf("%d", summary.Succeeded),
				"failed":    fmt.Sprintf("%d", summary.Failed),
			},
			SourceEvent: "bulk.completed",
			ActionURL:   &actionURL,
		})
	}
	if s.activity != nil {
		uid := job.ActorID
		s.activity.Record(ctx, &uid, "bulk.completed", job.Resource, &job.Uuid, map[string]any{
			"action": job.Action, "total": summary.Total,
			"succeeded": summary.Succeeded, "failed": summary.Failed,
		}, nil)
	}
	return nil
}

// Rollback reverts a completed job within its window. access decides which jobs the caller
// reaches; account-scoped adapters re-check the caller's rights for every reverted item.
func (s *Service) Rollback(ctx context.Context, jobUUID uuid.UUID, actorID int64, access JobAccess) (BulkJobView, error) {
	job, err := s.q.GetBulkJobByUUID(ctx, jobUUID)
	if err != nil || !access.allows(job, actorID) {
		return BulkJobView{}, ErrNotFound
	}
	ctx = bulkengine.WithActor(ctx, actorID)
	if job.AccountID.Valid {
		ctx = jobscope.With(ctx, jobscope.Scope{
			AccountID: job.AccountID.Int64, ActorID: actorID, Params: job.ParamsJson,
		})
	}
	if job.Status != "completed" {
		return BulkJobView{}, ErrInvalidRequest
	}
	if job.RollbackUntil.Valid && time.Now().After(job.RollbackUntil.Time) {
		return BulkJobView{}, ErrInvalidRequest
	}
	_, adapter, err := s.registry.ActionDef(job.Resource, job.Action)
	if err != nil {
		return BulkJobView{}, err
	}
	changes, err := s.q.ListBulkChangesForJob(ctx, job.ID)
	if err != nil {
		return BulkJobView{}, err
	}
	failed := 0
	for i := len(changes) - 1; i >= 0; i-- {
		ch := changes[i]
		var prev map[string]any
		_ = json.Unmarshal(ch.PreviousJson, &prev)
		if err := adapter.RevertItem(ctx, job.Action, ch.EntityUuid.String(), prev); err != nil {
			failed++
		}
	}
	status := "rolled_back"
	if failed > 0 {
		status = "rolled_back_partial"
	}
	row, err := s.q.MarkBulkJobRolledBack(ctx, db.MarkBulkJobRolledBackParams{
		ID: job.ID, Status: status,
	})
	if err != nil {
		return BulkJobView{}, err
	}
	if s.activity != nil {
		uid := actorID
		s.activity.Record(ctx, &uid, "bulk.rolled_back", job.Resource, &job.Uuid, map[string]any{
			"failed_rollback": failed,
		}, nil)
	}
	return mapBulkJob(row), nil
}

func (s *Service) ListJobs(ctx context.Context, actorID int64, admin bool, limit, offset int32) ([]BulkJobView, int64, error) {
	var rows []db.BulkJob
	var total int64
	var err error
	if admin {
		rows, err = s.q.ListAllBulkJobs(ctx, db.ListAllBulkJobsParams{LimitCount: limit, OffsetCount: offset})
		total, _ = s.q.CountAllBulkJobs(ctx)
	} else {
		rows, err = s.q.ListBulkJobsForActor(ctx, db.ListBulkJobsForActorParams{
			ActorID: actorID, LimitCount: limit, OffsetCount: offset,
		})
		total, _ = s.q.CountBulkJobsForActor(ctx, actorID)
	}
	if err != nil {
		return nil, 0, err
	}
	out := make([]BulkJobView, 0, len(rows))
	for _, r := range rows {
		out = append(out, mapBulkJob(r))
	}
	return out, total, nil
}

// ListAccountJobs lists one account's app jobs. Without access.AnyActor only the caller's own.
func (s *Service) ListAccountJobs(ctx context.Context, actorID int64, access JobAccess, resource string, limit, offset int32) ([]BulkJobView, int64, error) {
	if access.AccountID <= 0 {
		return nil, 0, ErrForbidden
	}
	account := pgtype.Int8{Int64: access.AccountID, Valid: true}
	params := db.ListBulkJobsForAccountParams{AccountID: account, LimitCount: limit, OffsetCount: offset}
	count := db.CountBulkJobsForAccountParams{AccountID: account}
	if !access.AnyActor {
		params.ActorID = pgtype.Int8{Int64: actorID, Valid: true}
		count.ActorID = params.ActorID
	}
	if resource != "" {
		params.Resource = pgtype.Text{String: resource, Valid: true}
		count.Resource = params.Resource
	}
	rows, err := s.q.ListBulkJobsForAccount(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountBulkJobsForAccount(ctx, count)
	if err != nil {
		return nil, 0, err
	}
	out := make([]BulkJobView, 0, len(rows))
	for _, r := range rows {
		out = append(out, mapBulkJob(r))
	}
	return out, total, nil
}

// GetAccountJob returns one job the caller reaches through access (404 otherwise).
func (s *Service) GetAccountJob(ctx context.Context, jobUUID uuid.UUID, actorID int64, access JobAccess) (BulkJobView, error) {
	row, err := s.q.GetBulkJobByUUID(ctx, jobUUID)
	if err != nil || !access.allows(row, actorID) {
		return BulkJobView{}, ErrNotFound
	}
	return mapBulkJob(row), nil
}

func (s *Service) GetJob(ctx context.Context, jobUUID uuid.UUID, actorID int64, admin bool) (BulkJobView, error) {
	row, err := s.q.GetBulkJobByUUID(ctx, jobUUID)
	if err != nil {
		return BulkJobView{}, ErrNotFound
	}
	if !admin && row.ActorID != actorID {
		return BulkJobView{}, ErrForbidden
	}
	return mapBulkJob(row), nil
}

func (s *Service) applyTargets(
	ctx context.Context,
	jobID int64,
	adapter bulkengine.BulkAdapter,
	def bulkengine.BulkActionDef,
	action string,
	targets []string,
) (bulkengine.BulkSummary, error) {
	summary := bulkengine.BulkSummary{Total: len(targets)}
	for _, entityUUID := range targets {
		res, err := adapter.ApplyItem(ctx, action, entityUUID)
		if err != nil {
			s.log.Warn("bulk_item_failed", "action", def.ID, "entity", entityUUID, "error", err)
			summary.AddError(entityUUID, "unexpected error")
			continue
		}
		if !res.OK {
			summary.AddError(entityUUID, res.Error)
			continue
		}
		summary.Succeeded++
		if jobID > 0 && def.Reversible {
			prev, _ := json.Marshal(res.Previous)
			_, _ = s.q.InsertBulkChange(ctx, db.InsertBulkChangeParams{
				JobID: jobID, EntityType: res.EntityType,
				EntityUuid: uuid.MustParse(res.EntityUUID), Op: res.Op,
				PreviousJson: prev,
			})
		}
	}
	return summary, nil
}

func (s *Service) failBulk(ctx context.Context, jobID int64, msg string) error {
	job, err := s.q.GetBulkJobByID(ctx, jobID)
	if err != nil {
		return err
	}
	if _, err := s.q.MarkBulkJobFailed(ctx, db.MarkBulkJobFailedParams{ID: jobID, Error: pgtype.Text{String: msg, Valid: true}}); err != nil {
		return err
	}
	if s.notifier != nil {
		uid := job.ActorID
		loc := i18n.Normalize(job.Locale)
		_, _ = s.notifier.Enqueue(ctx, notifmodel.EnqueueInput{
			UserID: &uid, Channels: []string{notifmodel.ChannelInapp},
			TemplateCode: "bulk.failed", Language: job.Locale,
			TemplateVars: map[string]string{
				"resource": i18n.ResourceLabel(loc, job.Resource),
				"action":   job.Action,
				"error":    msg,
			},
			SourceEvent: "bulk.failed",
		})
	}
	if s.activity != nil {
		uid := job.ActorID
		s.activity.Record(ctx, &uid, "bulk.failed", job.Resource, &job.Uuid, map[string]any{
			"action": job.Action, "error": msg,
		}, nil)
	}
	return fmt.Errorf("%s", msg)
}

func mapBulkJob(row db.BulkJob) BulkJobView {
	view := BulkJobView{
		UUID: row.Uuid, Resource: row.Resource, Action: row.Action,
		Status: row.Status, CreatedAt: row.CreatedAt.Time,
	}
	if len(row.ParamsJson) > 0 && !bytes.Equal(row.ParamsJson, []byte("{}")) {
		view.Params = json.RawMessage(row.ParamsJson)
	}
	if len(row.TargetJson) > 0 && !bytes.Equal(row.TargetJson, []byte("{}")) {
		var target bulkengine.BulkTarget
		_ = json.Unmarshal(row.TargetJson, &target)
		view.Target = target
	}
	if len(row.ResultJson) > 0 && !bytes.Equal(row.ResultJson, []byte("{}")) {
		var summary bulkengine.BulkSummary
		_ = json.Unmarshal(row.ResultJson, &summary)
		view.Summary = summary
	}
	if row.Error.Valid {
		view.Error = &row.Error.String
	}
	if row.RollbackUntil.Valid {
		t := row.RollbackUntil.Time
		view.RollbackUntil = &t
	}
	if row.AppliedAt.Valid {
		t := row.AppliedAt.Time
		view.AppliedAt = &t
	}
	return view
}

// Registry exposes the bulk adapter registry for handlers.
func (s *Service) Registry() *bulkengine.Registry {
	return s.registry
}
