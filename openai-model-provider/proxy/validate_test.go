package proxy

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func captureLogOutput(t *testing.T, fn func()) string {
	t.Helper()

	origWriter := log.Writer()
	origFlags := log.Flags()
	origPrefix := log.Prefix()

	var b bytes.Buffer
	log.SetOutput(&b)
	log.SetFlags(0)
	log.SetPrefix("")

	defer func() {
		log.SetOutput(origWriter)
		log.SetFlags(origFlags)
		log.SetPrefix(origPrefix)
	}()

	fn()
	return b.String()
}

func TestValidateEndpoint(t *testing.T) {
	t.Run("success returns 200", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"test-model"}]}`))
		}))
		t.Cleanup(ts.Close)

		cfg := &Config{BaseURL: ts.URL, APIKey: "test-key"}
		handler := http.NewServeMux()
		s := &server{cfg: cfg}
		handler.HandleFunc("GET /validate", s.validate)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/validate", nil)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %q", rr.Code, rr.Body.String())
		}
	})

	t.Run("validation error returns 400 with JSON error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"Incorrect API key provided: bad-key"}}`))
		}))
		t.Cleanup(ts.Close)

		cfg := &Config{BaseURL: ts.URL, APIKey: "bad-key"}
		handler := http.NewServeMux()
		s := &server{cfg: cfg}
		handler.HandleFunc("GET /validate", s.validate)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/validate", nil)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rr.Code)
		}
		if got := rr.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("expected Content-Type %q, got %q", "application/json", got)
		}

		var body map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decode response body: %v", err)
		}
		if got := body["error"]; got != "Invalid API Key" {
			t.Fatalf("expected validation message %q, got %q", "Invalid API Key", got)
		}
	})
}

func TestValidate_ErrorResponses(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody string
		expectedErr  string
		expectedLogs []string
	}{
		{
			name:         "uses upstream invalid key message",
			statusCode:   http.StatusUnauthorized,
			responseBody: `{"error":{"message":"Incorrect API key provided: bad-key"}}`,
			expectedErr:  "Invalid API Key",
			expectedLogs: []string{
				"ERROR Invalid: status 401 Unauthorized",
				"Incorrect API key provided: bad-key",
			},
		},
		{
			name:         "falls back to status when body not parseable",
			statusCode:   http.StatusBadGateway,
			responseBody: "not-json",
			expectedErr:  "status: 502 Bad Gateway",
			expectedLogs: []string{
				"ERROR Invalid: status 502 Bad Gateway: not-json",
			},
		},
		{
			name:         "uses parseable non-auth upstream error message",
			statusCode:   http.StatusTooManyRequests,
			responseBody: `{"error":{"message":"rate limit exceeded"}}`,
			expectedErr:  "rate limit exceeded",
			expectedLogs: []string{
				"ERROR Invalid: status 429 Too Many Requests",
				"rate limit exceeded",
			},
		},
		{
			name:         "maps invalid x-api-key to invalid api key",
			statusCode:   http.StatusUnauthorized,
			responseBody: `{"error":{"message":"invalid x-api-key"}}`,
			expectedErr:  "Invalid API Key",
			expectedLogs: []string{
				"ERROR Invalid: status 401 Unauthorized",
				"invalid x-api-key",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			t.Cleanup(ts.Close)

			cfg := &Config{BaseURL: ts.URL, APIKey: "test-key"}

			var err error
			logs := captureLogOutput(t, func() {
				err = cfg.Validate()
			})

			if err == nil {
				t.Fatal("expected validation error")
			}
			if got := err.Error(); got != tt.expectedErr {
				t.Fatalf("unexpected returned error: %q", got)
			}
			for _, expectedLog := range tt.expectedLogs {
				if !strings.Contains(logs, expectedLog) {
					t.Fatalf("expected log to contain %q, got: %q", expectedLog, logs)
				}
			}
		})
	}
}
