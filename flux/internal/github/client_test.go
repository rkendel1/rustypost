package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type staticTokenProvider struct{}

func (staticTokenProvider) LoadToken() (string, error) { return "test-token", nil }

func TestListRepositories(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedAuth := "Bearer " + "test-token"
		if r.Header.Get("Authorization") != expectedAuth {
			t.Fatalf("missing auth header")
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "name": "demo", "full_name": "acme/demo", "default_branch": "main"},
		})
	}))
	defer srv.Close()

	c := NewClient(staticTokenProvider{}).WithBaseURL(srv.URL)
	repos, err := c.ListRepositories(context.Background(), "")
	if err != nil {
		t.Fatalf("list repositories: %v", err)
	}
	if len(repos) != 1 || repos[0].FullName != "acme/demo" {
		t.Fatalf("unexpected repositories: %#v", repos)
	}
}

func TestCreatePullRequestAndListWorkflowRuns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/demo/pulls":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number": 22, "title": "Automation", "state": "open", "html_url": "https://example/pr/22",
			})
		case "/repos/acme/demo/actions/runs":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"workflow_runs": []map[string]any{{"id": 10, "name": "CI", "status": "completed", "conclusion": "success"}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient(staticTokenProvider{}).WithBaseURL(srv.URL)
	pr, err := c.CreatePullRequest(context.Background(), "acme", "demo", CreatePullRequestInput{
		Title: "Automation",
		Head:  "feature/automation",
		Base:  "main",
	})
	if err != nil {
		t.Fatalf("create pull request: %v", err)
	}
	if pr.Number != 22 {
		t.Fatalf("unexpected pull request: %#v", pr)
	}
	runs, err := c.ListWorkflowRuns(context.Background(), "acme", "demo", 10)
	if err != nil {
		t.Fatalf("list workflow runs: %v", err)
	}
	if len(runs) != 1 || runs[0].Name != "CI" {
		t.Fatalf("unexpected workflow runs: %#v", runs)
	}
}
