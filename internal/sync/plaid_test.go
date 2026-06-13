package sync

import (
	"errors"
	"net/http"
	"testing"
)

func TestIsPlaidTransientError(t *testing.T) {
	fakeResp := func(code int) *http.Response {
		return &http.Response{StatusCode: code}
	}

	tests := []struct {
		name     string
		err      error
		httpResp *http.Response
		want     bool
	}{
		{"nil error", nil, nil, false},
		{"network error no response", errors.New("connection refused"), nil, true},
		{"network error 500 response", errors.New("transport error"), fakeResp(500), true},
		{"network error 503 response", errors.New("transport error"), fakeResp(503), true},
		{"error with 400 response", errors.New("bad request"), fakeResp(400), false},
		{"error with 401 response", errors.New("unauthorized"), fakeResp(401), false},
		{"error with 429 response", errors.New("rate limited"), fakeResp(429), false},
		{"PlaidItemError is not transient", &PlaidItemError{Message: "item disconnected", Err: errors.New("ITEM_LOGIN_REQUIRED")}, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPlaidTransientError(tt.err, tt.httpResp)
			if got != tt.want {
				t.Errorf("IsPlaidTransientError() = %v, want %v", got, tt.want)
			}
		})
	}
}
