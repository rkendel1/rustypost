package main

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"flux/internal/automation/history"
	"flux/internal/automation/jobs"
	"flux/internal/scanner"
)

// repoScanJobPayload is the Payload carried by a jobs.TypeRepositoryScan
// job — enough for handleRepositoryScanJob to run without needing access to
// *App itself (Handler is a plain function type, not a method).
type repoScanJobPayload struct {
	RepoPath  string
	OutputDir string
}

// handleRepositoryScanJob wraps the existing internal/scanner package (also
// used by the CLI) as a jobs.Handler: scan the repository, then generate its
// OpenAPI/collection/test-suite artifacts. It doesn't reimplement any
// scanning logic — it only adapts scanner's two-call API to the job runner's
// progress/result contract.
func (a *App) handleRepositoryScanJob(ctx context.Context, job jobs.Job, progress func(int)) (any, error) {
	payload, ok := job.Payload.(repoScanJobPayload)
	if !ok {
		return nil, errors.New("invalid repository scan job payload")
	}
	progress(10)
	inv, err := scanner.ScanRepository(payload.RepoPath)
	if err != nil {
		return nil, err
	}
	progress(60)
	artifacts, err := scanner.GenerateArtifacts(payload.RepoPath, payload.OutputDir, inv)
	if err != nil {
		return nil, err
	}
	progress(100)
	return artifacts, nil
}

// isTerminalJobEvent reports whether a job event represents the end of a
// job's lifecycle — the only events worth persisting into activity history.
func isTerminalJobEvent(t jobs.EventType) bool {
	return t == jobs.EventCompleted || t == jobs.EventFailed || t == jobs.EventCancelled
}

// RunProjectScan enqueues a repository scan for a Project Source, returning
// the job ID immediately — the frontend observes progress via the
// "jobs:event" Wails event or GetProjectJobs.
func (a *App) RunProjectScan(projectID, sourceID string) (string, error) {
	if a.projects == nil || a.jobQueue == nil {
		return "", errors.New("not initialised")
	}
	repoPath, outputDir, err := a.resolveProjectSourceAndOutput(projectID, sourceID, "")
	if err != nil {
		return "", err
	}
	job := jobs.Job{
		ID:        uuid.NewString(),
		Type:      jobs.TypeRepositoryScan,
		ProjectID: projectID,
		SourceID:  sourceID,
		Payload:   repoScanJobPayload{RepoPath: repoPath, OutputDir: outputDir},
	}
	a.jobQueue.Enqueue(job)
	return job.ID, nil
}

// GetProjectJobs returns known jobs for a project (queued, running, or
// finished — Runner retains completed jobs in its in-memory queue state).
func (a *App) GetProjectJobs(projectID string) []jobs.Job {
	if a.jobQueue == nil {
		return nil
	}
	all := a.jobQueue.List()
	out := make([]jobs.Job, 0, len(all))
	for _, j := range all {
		if j.ProjectID == projectID {
			out = append(out, j)
		}
	}
	return out
}

// GetProjectActivity returns the active project's masker-redacted job
// activity timeline, most recent last.
func (a *App) GetProjectActivity(limit int) ([]history.Entry, error) {
	if a.automationHistory == nil {
		return nil, errors.New("no active project")
	}
	return a.automationHistory.List(limit)
}
