//go:build acceptance

package outsidemcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

type failedEventRPC struct{ err error }

func (e failedEventRPC) RPC(context.Context, string, string, json.RawMessage) (any, error) {
	return nil, e.err
}

type callbackVerificationError struct{ cause error }

func (e callbackVerificationError) Error() string {
	return fmt.Sprintf("Callback endpoint verification failed: %v", e.cause)
}
func (e callbackVerificationError) Unwrap() error { return e.cause }
func (callbackVerificationError) RPCError() (int, string, any) {
	return -32015, "Callback endpoint verification failed", map[string]string{"reason": "unreachable"}
}

func TestEventVerificationCauseIsReportedLocally(t *testing.T) {
	h, _ := fixture(t)
	cause := errors.New("synthetic TLS certificate rejected: unknown authority")
	h.config.Events = failedEventRPC{err: callbackVerificationError{cause: cause}}
	var diagnostics []error
	h.config.ReportError = func(err error) { diagnostics = append(diagnostics, err) }
	w := httptest.NewRecorder()
	h.ServeHTTP(w, request("events/subscribe", map[string]any{}))
	if len(diagnostics) != 1 || !errors.Is(diagnostics[0], cause) || !strings.Contains(diagnostics[0].Error(), cause.Error()) {
		t.Fatalf("missing local callback cause: %v", diagnostics)
	}
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"code":-32015`) || !strings.Contains(w.Body.String(), `"reason":"unreachable"`) || strings.Contains(w.Body.String(), cause.Error()) {
		t.Fatalf("unsafe public callback response: %d %s", w.Code, w.Body.String())
	}
}
