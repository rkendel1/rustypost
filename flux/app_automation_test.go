package main

import (
	"context"
	"testing"
	"time"

	"flux/internal/automation/events"
	automationhistory "flux/internal/automation/history"
	"flux/internal/automation/jobs"
	"flux/internal/automation/pipeline"
	"flux/internal/projects"
)

// TestRunProjectScanCompletesAndRecordsHistory exercises the Phase 6
// vertical slice: enqueuing a project-scoped repository scan job runs
// through the real jobs.Runner (wrapping internal/scanner), publishes a
// completion event carrying the ProjectID, and — once wired the same way
// app.go's startup() wires it — lands in the project's activity history.
func TestRunProjectScanCompletesAndRecordsHistory(t *testing.T) {
	root := t.TempDir()
	a := NewApp()
	a.ctx = context.Background()
	a.projects = projects.NewService(t.TempDir())

	a.jobQueue = jobs.NewQueue(8)
	a.jobEvents = jobs.NewPublisher()
	a.jobRunner = jobs.NewRunner(a.jobQueue, a.jobEvents)
	a.jobRunner.Register(jobs.TypeRepositoryScan, a.handleRepositoryScanJob)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.jobRunner.Start(ctx, 1)

	a.automationBus = events.NewBus()
	a.pipelineEngine = pipeline.NewEngine(a.automationBus)
	a.automationHistory = automationhistory.NewStore(root)

	// Mirror app.go's startup() event-forwarding goroutine, minus the Wails
	// runtime.EventsEmit call (there's no real Wails context in a test).
	jobEventCh, unsubscribe := a.jobEvents.Subscribe(16)
	defer unsubscribe()
	done := make(chan jobs.Job, 1)
	go func() {
		for evt := range jobEventCh {
			if !isTerminalJobEvent(evt.Type) {
				continue
			}
			_ = a.automationHistory.Append(automationhistory.Entry{
				ID:        evt.Job.ID,
				Kind:      string(evt.Job.Type),
				Name:      string(evt.Job.Type),
				Status:    string(evt.Job.Status),
				ProjectID: evt.Job.ProjectID,
				SourceID:  evt.Job.SourceID,
				JobID:     evt.Job.ID,
			})
			done <- evt.Job
			return
		}
	}()

	p, err := a.projects.Create(a.ctx, projects.CreateProjectInput{Name: "ScanMe", RootDir: root})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	src, err := a.projects.AddSource(a.ctx, p.ID, projects.AddSourceInput{Name: "root", Kind: projects.SourceTypeLocalFolder, Path: "."})
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}

	jobID, err := a.RunProjectScan(p.ID, src.ID)
	if err != nil {
		t.Fatalf("RunProjectScan: %v", err)
	}
	if jobID == "" {
		t.Fatal("expected a job ID")
	}

	select {
	case finished := <-done:
		if finished.ID != jobID {
			t.Fatalf("expected job ID %s, got %s", jobID, finished.ID)
		}
		if finished.ProjectID != p.ID {
			t.Errorf("expected job ProjectID %s, got %s", p.ID, finished.ProjectID)
		}
		if finished.Status != jobs.StatusCompleted {
			t.Fatalf("expected job to complete, got status=%s error=%s", finished.Status, finished.Error)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for scan job to finish")
	}

	entries, err := a.GetProjectActivity(0)
	if err != nil {
		t.Fatalf("GetProjectActivity: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.JobID == jobID && e.ProjectID == p.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the scan job to appear in project activity history, got %+v", entries)
	}

	jobsForProject := a.GetProjectJobs(p.ID)
	if len(jobsForProject) != 1 || jobsForProject[0].ID != jobID {
		t.Errorf("expected GetProjectJobs to return the scan job, got %+v", jobsForProject)
	}
}
