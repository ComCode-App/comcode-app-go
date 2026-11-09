package queue

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hibiken/asynq"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/logging"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/reqid"
)

func TestWithRequestIDPropagatesToHandlerContext(t *testing.T) {
	task, err := NewExportProcessTask(42)
	if err != nil {
		t.Fatal(err)
	}
	if got := WithRequestID(context.Background(), task); got != task {
		t.Fatal("task without request id in ctx should be returned unchanged")
	}

	tagged := WithRequestID(reqid.With(context.Background(), "rid-job-1"), task)
	if TaskRequestID(tagged) != "rid-job-1" {
		t.Fatalf("header = %v", tagged.Headers())
	}
	if tagged.Type() != task.Type() || !bytes.Equal(tagged.Payload(), task.Payload()) {
		t.Fatal("type/payload must be preserved")
	}

	var seen string
	h := observeMiddleware(asynq.HandlerFunc(func(ctx context.Context, _ *asynq.Task) error {
		seen = reqid.From(ctx)
		return nil
	}))
	if err := h.ProcessTask(context.Background(), tagged); err != nil {
		t.Fatal(err)
	}
	if seen != "rid-job-1" {
		t.Fatalf("handler ctx request id = %q", seen)
	}
}

func TestTaskRequestIDRejectsInvalidHeader(t *testing.T) {
	task := asynq.NewTaskWithHeaders(TaskPing, nil, map[string]string{reqid.Header: "bad id"})
	if TaskRequestID(task) != "" {
		t.Fatal("invalid header must be ignored")
	}
}

func TestErrorHandlerLogsTaskAndRequestID(t *testing.T) {
	var buf bytes.Buffer
	log := logging.NewWithWriter(&buf, "debug", "json")
	task := asynq.NewTaskWithHeaders(TaskBulkProcess, []byte(`{}`), map[string]string{reqid.Header: "rid-job-2"})
	errorHandler(log).HandleError(context.Background(), task, errors.New("db down"))
	out := buf.String()
	for _, want := range []string{`"msg":"queue_task_failed"`, `"type":"app:bulk:process"`, `"request_id":"rid-job-2"`, `"error":"db down"`, `"retry":0`} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %s\n%s", want, out)
		}
	}
}
