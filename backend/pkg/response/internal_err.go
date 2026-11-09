package response

import "net/http"

// StatusClientClosedRequest is the nginx convention for "the client went away before the
// response was written". It is only used in logs and metrics; nothing reaches the client.
const StatusClientClosedRequest = 499

type serverErrorRecorder interface {
	RecordServerError(err error, publicMessage string)
}

type errorCodeRecorder interface {
	RecordErrorCode(code string)
}

type failureRecorder interface {
	RecordFailure(err error)
}

// InternalErr writes a generic 500 response and records err for server-side logging.
func InternalErr(w http.ResponseWriter, r *http.Request, err error, publicMessage string) {
	if err != nil {
		if rec, ok := w.(serverErrorRecorder); ok {
			rec.RecordServerError(err, publicMessage)
		}
	}
	Internal(w, r, publicMessage)
}

// RecordFailure keeps the cause of a non-5xx failure (e.g. why a sign-in was rejected)
// for the request log. The request observer redacts it before logging; it never reaches
// the client. A no-op when w is not the observer's writer.
func RecordFailure(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	if rec, ok := w.(failureRecorder); ok {
		rec.RecordFailure(err)
	}
}
