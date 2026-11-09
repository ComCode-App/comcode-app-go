package metrics

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestWriteExposition(t *testing.T) {
	ObserveHTTP("get", 200, 20*time.Millisecond)
	ObserveHTTP("BREW", 503, 2*time.Second)
	ObserveTask("app:ping", nil, time.Millisecond)
	ObserveTask("app:ping", errors.New("boom"), time.Millisecond)

	var b strings.Builder
	Write(&b)
	out := b.String()
	for _, want := range []string{
		`http_requests_total{method="GET",code="2xx"}`,
		`http_requests_total{method="OTHER",code="5xx"}`,
		`http_request_duration_seconds_bucket{le="+Inf"}`,
		`queue_tasks_total{type="app:ping",result="success"} 1`,
		`queue_tasks_total{type="app:ping",result="failure"} 1`,
		"# TYPE go_goroutines gauge",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("exposition missing %q\n%s", want, out)
		}
	}
}
