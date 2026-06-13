package sync

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestIsProviderTransientError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "teller 5xx is transient",
			err:  &TellerAPIError{StatusCode: http.StatusInternalServerError},
			want: true,
		},
		{
			name: "teller 4xx is NOT transient",
			err:  &TellerAPIError{StatusCode: http.StatusBadRequest},
			want: false,
		},
		{
			name: "teller 422 is NOT transient",
			err:  &TellerAPIError{StatusCode: http.StatusUnprocessableEntity},
			want: false,
		},
		{
			name: "akahu 5xx is transient",
			err:  &AkahuAPIError{StatusCode: http.StatusInternalServerError},
			want: true,
		},
		{
			name: "akahu 400 is NOT transient",
			err:  &AkahuAPIError{StatusCode: http.StatusBadRequest},
			want: false,
		},
		{
			name: "generic error is NOT transient",
			err:  errors.New("some unexpected error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsProviderTransientError(tt.err)
			if got != tt.want {
				t.Errorf("IsProviderTransientError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestRetryWithBackoff(t *testing.T) {
	errFail := errors.New("fail")

	t.Run("succeeds on first attempt", func(t *testing.T) {
		calls := 0
		err := retryWithBackoff(context.Background(), 3, func() (bool, error) {
			calls++
			return false, nil
		})
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if calls != 1 {
			t.Fatalf("expected 1 call, got %d", calls)
		}
	})

	t.Run("returns non-retryable error immediately", func(t *testing.T) {
		calls := 0
		err := retryWithBackoff(context.Background(), 3, func() (bool, error) {
			calls++
			return false, errFail
		})
		if !errors.Is(err, errFail) {
			t.Fatalf("expected errFail, got %v", err)
		}
		if calls != 1 {
			t.Fatalf("expected 1 call, got %d", calls)
		}
	})

	t.Run("exhausts retries and returns last error", func(t *testing.T) {
		calls := 0
		// Use maxRetries=0 so no actual sleep occurs but retry is tested.
		err := retryWithBackoff(context.Background(), 0, func() (bool, error) {
			calls++
			return true, errFail
		})
		if !errors.Is(err, errFail) {
			t.Fatalf("expected errFail, got %v", err)
		}
		if calls != 1 {
			t.Fatalf("expected 1 call (maxRetries=0), got %d", calls)
		}
	})

	t.Run("succeeds after retries", func(t *testing.T) {
		calls := 0
		err := retryWithBackoff(context.Background(), 2, func() (bool, error) {
			calls++
			if calls < 3 {
				return true, errFail
			}
			return false, nil
		})
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if calls != 3 {
			t.Fatalf("expected 3 calls, got %d", calls)
		}
	})

	t.Run("context cancelled during backoff returns ctx error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		err := retryWithBackoff(ctx, 1, func() (bool, error) {
			calls++
			cancel() // cancel context so next iteration's sleep exits immediately
			return true, errFail
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})
}
