package github

import (
	"context"
	"fmt"
	"net/url"
)

type WorkflowRun struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HTMLURL    string `json:"html_url"`
	HeadBranch string `json:"head_branch"`
}

type workflowRunsResponse struct {
	WorkflowRuns []WorkflowRun `json:"workflow_runs"`
}

func (c *Client) ListWorkflowRuns(ctx context.Context, owner, repo string, perPage int) ([]WorkflowRun, error) {
	if perPage <= 0 {
		perPage = 20
	}
	q := url.Values{}
	q.Set("per_page", fmt.Sprintf("%d", perPage))
	path := fmt.Sprintf("/repos/%s/%s/actions/runs?%s", owner, repo, q.Encode())
	var out workflowRunsResponse
	if err := c.getJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	return out.WorkflowRuns, nil
}
