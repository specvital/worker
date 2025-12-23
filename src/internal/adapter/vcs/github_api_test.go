package vcs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/specvital/collector/internal/domain/analysis"
)

func TestNewGitHubAPIClient(t *testing.T) {
	t.Run("with nil http client uses default", func(t *testing.T) {
		client := NewGitHubAPIClient(nil)
		if client == nil {
			t.Fatal("expected non-nil client")
		}
		if client.httpClient != http.DefaultClient {
			t.Error("expected http.DefaultClient")
		}
	})

	t.Run("with custom http client", func(t *testing.T) {
		custom := &http.Client{}
		client := NewGitHubAPIClient(custom)
		if client.httpClient != custom {
			t.Error("expected custom http client")
		}
	})
}

func TestGitHubAPIClient_GetRepoID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/repos/octocat/Hello-World" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Header.Get("Accept") != "application/vnd.github+json" {
				t.Error("missing Accept header")
			}
			if r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
				t.Error("missing or incorrect API version header")
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id": 1296269}`))
		}))
		defer server.Close()

		client := newTestClient(server)

		id, err := client.GetRepoID(context.Background(), "github.com", "octocat", "Hello-World", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "1296269" {
			t.Errorf("expected id 1296269, got %s", id)
		}
	})

	t.Run("success with token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer test-token" {
				t.Errorf("unexpected Authorization header: %s", auth)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id": 12345}`))
		}))
		defer server.Close()

		client := newTestClient(server)
		token := "test-token"

		id, err := client.GetRepoID(context.Background(), "github.com", "owner", "repo", &token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "12345" {
			t.Errorf("expected id 12345, got %s", id)
		}
	})

	t.Run("empty token does not set auth header", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if auth := r.Header.Get("Authorization"); auth != "" {
				t.Errorf("expected no Authorization header, got %s", auth)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id": 12345}`))
		}))
		defer server.Close()

		client := newTestClient(server)
		emptyToken := ""

		_, err := client.GetRepoID(context.Background(), "github.com", "owner", "repo", &emptyToken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("repository not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := newTestClient(server)

		_, err := client.GetRepoID(context.Background(), "github.com", "owner", "nonexistent", nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, analysis.ErrRepoNotFound) {
			t.Errorf("expected ErrRepoNotFound, got %v", err)
		}
	})

	t.Run("unsupported host", func(t *testing.T) {
		client := NewGitHubAPIClient(nil)
		_, err := client.GetRepoID(context.Background(), "gitlab.com", "owner", "repo", nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, analysis.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("empty owner", func(t *testing.T) {
		client := NewGitHubAPIClient(nil)
		_, err := client.GetRepoID(context.Background(), "github.com", "", "repo", nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, analysis.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("empty repo", func(t *testing.T) {
		client := NewGitHubAPIClient(nil)
		_, err := client.GetRepoID(context.Background(), "github.com", "owner", "", nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, analysis.ErrInvalidInput) {
			t.Errorf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := newTestClient(server)

		_, err := client.GetRepoID(context.Background(), "github.com", "owner", "repo", nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "unexpected status 500") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id": 1}`))
		}))
		defer server.Close()

		client := newTestClient(server)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := client.GetRepoID(ctx, "github.com", "owner", "repo", nil)
		if err == nil {
			t.Fatal("expected context cancellation error")
		}
	})

	t.Run("invalid json response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`invalid json`))
		}))
		defer server.Close()

		client := newTestClient(server)

		_, err := client.GetRepoID(context.Background(), "github.com", "owner", "repo", nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "decode response") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func newTestClient(server *httptest.Server) *GitHubAPIClient {
	return &GitHubAPIClient{
		apiBase:    server.URL,
		httpClient: server.Client(),
	}
}
