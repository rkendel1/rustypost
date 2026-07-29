package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type TokenProvider interface {
	LoadToken() (string, error)
}

type Client struct {
	baseURL    string
	httpClient *http.Client
	tokens     TokenProvider
}

func NewClient(tokens TokenProvider) *Client {
	return &Client{
		baseURL:    "https://api.github.com",
		httpClient: http.DefaultClient,
		tokens:     tokens,
	}
}

func (c *Client) WithBaseURL(base string) *Client {
	copy := *c
	copy.baseURL = strings.TrimRight(base, "/")
	return &copy
}

func (c *Client) WithHTTPClient(h *http.Client) *Client {
	copy := *c
	copy.httpClient = h
	return &copy
}

type Repository struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	CloneURL      string `json:"clone_url"`
	HTMLURL       string `json:"html_url"`
}

type Viewer struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
}

func (c *Client) GetViewer(ctx context.Context) (*Viewer, error) {
	var out Viewer
	if err := c.getJSON(ctx, "/user", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListRepositories(ctx context.Context, visibility string) ([]Repository, error) {
	q := url.Values{}
	q.Set("per_page", "100")
	if visibility != "" {
		q.Set("visibility", visibility)
	}
	var repos []Repository
	if err := c.getJSON(ctx, "/user/repos?"+q.Encode(), &repos); err != nil {
		return nil, err
	}
	return repos, nil
}

func (c *Client) GetRepository(ctx context.Context, owner, repo string) (*Repository, error) {
	var out Repository
	if err := c.getJSON(ctx, fmt.Sprintf("/repos/%s/%s", owner, repo), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	token, err := c.tokens.LoadToken()
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github api %s: %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
