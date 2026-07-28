package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"flux/internal/mock"
	"flux/internal/scanner"
	"flux/internal/watcher"
)

type scanConfig struct {
	repoPath  string
	outputDir string
}

type healthCategory struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
	Score  float64 `json:"score"`
}

type healthReport struct {
	GeneratedAt string           `json:"generatedAt"`
	Repository  string           `json:"repository"`
	Overall     float64          `json:"overall"`
	Categories  []healthCategory `json:"categories"`
}

func loginCommand(args []string) int {
	if len(args) < 1 || args[0] != "github" {
		fmt.Fprintln(os.Stderr, "Usage: reqit login github [--token <token>]")
		return 1
	}
	token := ""
	for i := 1; i < len(args); i++ {
		if args[i] == "--token" && i+1 < len(args) {
			i++
			token = args[i]
		}
	}
	if strings.TrimSpace(token) == "" {
		token = strings.TrimSpace(os.Getenv("REQIT_GITHUB_TOKEN"))
	}
	if token == "" {
		fmt.Fprintln(os.Stderr, "GitHub token required. Provide --token or REQIT_GITHUB_TOKEN.")
		return 1
	}
	if err := saveGitHubToken(token); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save token: %v\n", err)
		return 1
	}
	fmt.Println("GitHub token saved.")
	return 0
}

func listReposCommand(_ []string) int {
	token, err := loadGitHubToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "GitHub login required: %v\n", err)
		return 1
	}
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/user/repos?per_page=100&sort=updated", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build request: %v\n", err)
		return 1
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GitHub request failed: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		fmt.Fprintf(os.Stderr, "GitHub request failed: %s: %s\n", resp.Status, strings.TrimSpace(string(b)))
		return 1
	}

	var repos []struct {
		FullName string `json:"full_name"`
		Private  bool   `json:"private"`
		HTMLURL  string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse GitHub response: %v\n", err)
		return 1
	}
	for _, r := range repos {
		visibility := "public"
		if r.Private {
			visibility = "private"
		}
		fmt.Printf("%s (%s)\n  %s\n", r.FullName, visibility, r.HTMLURL)
	}
	return 0
}

func cloneRepoCommand(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: reqit clone <owner/repository> [--dir <directory>]")
		return 1
	}
	slug := args[0]
	targetDir := ""
	for i := 1; i < len(args); i++ {
		if args[i] == "--dir" && i+1 < len(args) {
			i++
			targetDir = args[i]
		}
	}
	if targetDir == "" {
		parts := strings.Split(slug, "/")
		targetDir = parts[len(parts)-1]
	}

	url := slug
	if !strings.Contains(slug, "://") {
		url = "https://github.com/" + strings.TrimSuffix(strings.TrimPrefix(slug, "/"), ".git") + ".git"
	}
	if _, err := gitExec("", "clone", url, targetDir); err != nil {
		fmt.Fprintf(os.Stderr, "Clone failed: %v\n", err)
		return 1
	}
	fmt.Printf("Cloned %s -> %s\n", slug, targetDir)
	return 0
}

func watchRepository(args []string) int {
	cfg, err := parseScanConfig(args, "Usage: reqit watch <repository-path> [--output <dir>]")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	run := func(reason string) {
		inv, scanErr := scanner.ScanRepository(cfg.repoPath)
		if scanErr != nil {
			fmt.Fprintf(os.Stderr, "Scan failed (%s): %v\n", reason, scanErr)
			return
		}
		if _, genErr := scanner.GenerateArtifacts(cfg.repoPath, cfg.outputDir, inv); genErr != nil {
			fmt.Fprintf(os.Stderr, "Artifact generation failed (%s): %v\n", reason, genErr)
			return
		}
		fmt.Printf("[%s] scanned %d files, %d endpoints\n", time.Now().Format(time.RFC3339), inv.FilesScanned, len(inv.Endpoints))
	}
	run("initial")

	dirs, err := listWatchDirs(cfg.repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list directories: %v\n", err)
		return 1
	}

	w, err := watcher.New(func(changed string) {
		run(filepath.ToSlash(changed))
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Watcher start failed: %v\n", err)
		return 1
	}
	defer w.Close()
	if err := w.Watch(dirs...); err != nil {
		fmt.Fprintf(os.Stderr, "Watcher setup failed: %v\n", err)
		return 1
	}

	fmt.Printf("Watching %s (Ctrl+C to stop)\n", cfg.repoPath)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	fmt.Println("Watch stopped.")
	return 0
}

func generateCommand(args []string) int {
	cfg, err := parseScanConfig(args, "Usage: reqit generate <repository-path> [--output <dir>]")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	_, artifacts, err := scanAndGenerate(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Generate failed: %v\n", err)
		return 1
	}
	fmt.Printf("Generated artifacts in %s\n", artifacts.OutputDir)
	return 0
}

func pushCommand(args []string) int {
	repo := "."
	for i := 0; i < len(args); i++ {
		if args[i] == "--repo" && i+1 < len(args) {
			i++
			repo = args[i]
		}
	}
	out, err := gitExec(repo, "push")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Push failed: %v\n", err)
		return 1
	}
	if strings.TrimSpace(out) != "" {
		fmt.Println(out)
	}
	return 0
}

func syncCommand(args []string) int {
	repo := "."
	output := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			if i+1 < len(args) {
				i++
				repo = args[i]
			}
		case "--output", "-o":
			if i+1 < len(args) {
				i++
				output = args[i]
			}
		}
	}
	if _, err := gitExec(repo, "pull", "--rebase"); err != nil {
		fmt.Fprintf(os.Stderr, "Sync failed during pull: %v\n", err)
		return 1
	}
	cfg, err := parseScanConfig([]string{repo, "--output", output}, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Sync failed: %v\n", err)
		return 1
	}
	inv, _, err := scanAndGenerate(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Sync failed during generate: %v\n", err)
		return 1
	}
	fmt.Printf("Synced and regenerated %d endpoints.\n", len(inv.Endpoints))
	return 0
}

func healthCommand(args []string) int {
	cfg, err := parseScanConfig(args, "Usage: reqit health <repository-path> [--output <dir>]")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	inv, artifacts, err := scanAndGenerate(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Health failed: %v\n", err)
		return 1
	}
	report := computeHealth(inv, artifacts)
	b, _ := json.MarshalIndent(report, "", "  ")
	if err := os.WriteFile(filepath.Join(artifacts.OutputDir, "health.json"), b, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed writing health report: %v\n", err)
		return 1
	}
	fmt.Println(string(b))
	return 0
}

func driftCommand(args []string) int {
	cfg, err := parseScanConfig(args, "Usage: reqit drift <repository-path> [--output <dir>]")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	_, artifacts, err := scanAndGenerate(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Drift failed: %v\n", err)
		return 1
	}
	b, err := os.ReadFile(artifacts.DriftPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed reading drift report: %v\n", err)
		return 1
	}
	fmt.Println(string(b))
	return 0
}

func workflowCommand(args []string) int {
	if len(args) < 1 || args[0] != "install" {
		fmt.Fprintln(os.Stderr, "Usage: reqit workflow install <repository-path> [--output <dir>]")
		return 1
	}
	cfg, err := parseScanConfig(args[1:], "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Usage: reqit workflow install <repository-path> [--output <dir>]")
		return 1
	}
	_, artifacts, err := scanAndGenerate(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Workflow install failed: %v\n", err)
		return 1
	}
	paths, err := writeWorkflowFiles(cfg.repoPath, artifacts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Workflow install failed: %v\n", err)
		return 1
	}
	for _, p := range paths {
		fmt.Printf("Installed %s\n", p)
	}
	return 0
}

func sdkCommand(args []string) int {
	if len(args) < 1 || args[0] != "generate" {
		fmt.Fprintln(os.Stderr, "Usage: reqit sdk generate <repository-path> [--output <dir>]")
		return 1
	}
	cfg, err := parseScanConfig(args[1:], "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Usage: reqit sdk generate <repository-path> [--output <dir>]")
		return 1
	}
	inv, artifacts, err := scanAndGenerate(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SDK generation failed: %v\n", err)
		return 1
	}
	sdkDir := filepath.Join(artifacts.OutputDir, "sdk")
	if err := os.MkdirAll(sdkDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "SDK generation failed: %v\n", err)
		return 1
	}
	if err := writeSDKFiles(sdkDir, inv); err != nil {
		fmt.Fprintf(os.Stderr, "SDK generation failed: %v\n", err)
		return 1
	}
	fmt.Printf("SDKs generated in %s\n", sdkDir)
	return 0
}

func mockCommand(args []string) int {
	if len(args) < 1 || args[0] != "start" {
		fmt.Fprintln(os.Stderr, "Usage: reqit mock start <repository-path> [--port <port>]")
		return 1
	}
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: reqit mock start <repository-path> [--port <port>]")
		return 1
	}
	repo := args[1]
	port := 4010
	for i := 2; i < len(args); i++ {
		if args[i] == "--port" && i+1 < len(args) {
			i++
			if p, err := strconv.Atoi(args[i]); err == nil && p > 0 && p < 65536 {
				port = p
			}
		}
	}
	inv, err := scanner.ScanRepository(repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Mock start failed: %v\n", err)
		return 1
	}
	reg := mock.NewRegistry()
	for _, ep := range inv.Endpoints {
		reg.Set(ep.Method, ep.Path, mock.MockResponse{
			StatusCode: 200,
			Body: map[string]any{
				"ok":     true,
				"method": ep.Method,
				"path":   ep.Path,
			},
		})
	}
	ms := mock.NewMockServer(reg, port)
	ms.Start()
	fmt.Printf("Mock server running on http://localhost:%d with %d route(s). Ctrl+C to stop.\n", port, reg.Count())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	_ = ms.Stop()
	return 0
}

func reportCommand(args []string) int {
	cfg, err := parseScanConfig(args, "Usage: reqit report <repository-path> [--output <dir>]")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	inv, artifacts, err := scanAndGenerate(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Report failed: %v\n", err)
		return 1
	}
	drift, _ := os.ReadFile(artifacts.DriftPath)
	report := map[string]any{
		"generatedAt":   time.Now().UTC().Format(time.RFC3339),
		"repository":    inv.Repository,
		"filesScanned":  inv.FilesScanned,
		"endpointCount": len(inv.Endpoints),
		"authSchemes":   inv.AuthSchemes,
		"artifacts":     artifacts,
		"drift":         json.RawMessage(drift),
	}
	outJSON, _ := json.MarshalIndent(report, "", "  ")
	jsonPath := filepath.Join(artifacts.OutputDir, "report.json")
	mdPath := filepath.Join(artifacts.OutputDir, "report.md")
	htmlPath := filepath.Join(artifacts.OutputDir, "report.html")
	if err := os.WriteFile(jsonPath, outJSON, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed writing report.json: %v\n", err)
		return 1
	}
	md := fmt.Sprintf("# Reqit Repository Report\n\n- Generated: %s\n- Repository: `%s`\n- Files scanned: %d\n- Endpoints discovered: %d\n- Auth schemes: %s\n",
		time.Now().UTC().Format(time.RFC3339), inv.Repository, inv.FilesScanned, len(inv.Endpoints), strings.Join(inv.AuthSchemes, ", "))
	if err := os.WriteFile(mdPath, []byte(md), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed writing report.md: %v\n", err)
		return 1
	}
	html := fmt.Sprintf("<html><body><h1>Reqit Repository Report</h1><p><strong>Repository:</strong> %s</p><p><strong>Files scanned:</strong> %d</p><p><strong>Endpoints:</strong> %d</p></body></html>",
		inv.Repository, inv.FilesScanned, len(inv.Endpoints))
	if err := os.WriteFile(htmlPath, []byte(html), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed writing report.html: %v\n", err)
		return 1
	}
	fmt.Printf("Report generated:\n- %s\n- %s\n- %s\n", jsonPath, mdPath, htmlPath)
	return 0
}

func createPRCommand(args []string) int {
	title := ""
	body := ""
	base := "main"
	repoDir := "."
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--title":
			if i+1 < len(args) {
				i++
				title = args[i]
			}
		case "--body":
			if i+1 < len(args) {
				i++
				body = args[i]
			}
		case "--base":
			if i+1 < len(args) {
				i++
				base = args[i]
			}
		case "--repo":
			if i+1 < len(args) {
				i++
				repoDir = args[i]
			}
		}
	}
	if strings.TrimSpace(title) == "" {
		fmt.Fprintln(os.Stderr, "Usage: reqit create-pr --title <title> [--body <body>] [--base <branch>] [--repo <path>]")
		return 1
	}
	token, err := loadGitHubToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "GitHub login required: %v\n", err)
		return 1
	}
	owner, repo, err := parseRepoSlugFromRemote(repoDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to infer repository: %v\n", err)
		return 1
	}
	head, err := gitExec(repoDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to detect current branch: %v\n", err)
		return 1
	}
	payload := map[string]string{
		"title": title,
		"head":  strings.TrimSpace(head),
		"base":  base,
		"body":  body,
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", owner, repo), bytes.NewReader(b))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build GitHub request: %v\n", err)
		return 1
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Create PR failed: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		fmt.Fprintf(os.Stderr, "Create PR failed: %s: %s\n", resp.Status, strings.TrimSpace(string(respBody)))
		return 1
	}
	var out struct {
		HTMLURL string `json:"html_url"`
		Number  int    `json:"number"`
	}
	_ = json.Unmarshal(respBody, &out)
	fmt.Printf("Created PR #%d: %s\n", out.Number, out.HTMLURL)
	return 0
}

func parseScanConfig(args []string, usage string) (*scanConfig, error) {
	if len(args) < 1 {
		if usage != "" {
			return nil, errors.New(usage)
		}
		return nil, errors.New("repository path is required")
	}
	targetPath := ""
	outputDir := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--output", "-o":
			if i+1 < len(args) {
				i++
				outputDir = args[i]
			}
		default:
			if targetPath == "" {
				targetPath = args[i]
			}
		}
	}
	if targetPath == "" {
		if usage != "" {
			return nil, errors.New(usage)
		}
		return nil, errors.New("repository path is required")
	}
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, fmt.Errorf("error resolving path: %w", err)
	}
	if outputDir == "" {
		outputDir = filepath.Join(absTarget, ".reqit", "scan")
	} else if !filepath.IsAbs(outputDir) {
		outputDir = filepath.Join(absTarget, outputDir)
	}
	return &scanConfig{repoPath: absTarget, outputDir: outputDir}, nil
}

func scanAndGenerate(cfg *scanConfig) (*scanner.Inventory, *scanner.Artifacts, error) {
	inv, err := scanner.ScanRepository(cfg.repoPath)
	if err != nil {
		return nil, nil, err
	}
	artifacts, err := scanner.GenerateArtifacts(cfg.repoPath, cfg.outputDir, inv)
	if err != nil {
		return nil, nil, err
	}
	return inv, artifacts, nil
}

func computeHealth(inv *scanner.Inventory, artifacts *scanner.Artifacts) healthReport {
	endpointScore := 30.0
	if len(inv.Endpoints) == 0 {
		endpointScore = 5.0
	}
	authScore := 25.0
	if len(inv.AuthSchemes) == 0 {
		authScore = 8.0
	}
	openapiScore := 20.0
	if _, err := os.Stat(artifacts.OpenAPIPath); err != nil {
		openapiScore = 0
	}
	testScore := 15.0
	if _, err := os.Stat(artifacts.HarnessPath); err != nil {
		testScore = 0
	}
	driftScore := 10.0
	if _, err := os.Stat(artifacts.DriftPath); err != nil {
		driftScore = 0
	}
	categories := []healthCategory{
		{Name: "Endpoint Coverage", Weight: 30, Score: endpointScore},
		{Name: "Authentication", Weight: 25, Score: authScore},
		{Name: "OpenAPI Completeness", Weight: 20, Score: openapiScore},
		{Name: "Test Harness", Weight: 15, Score: testScore},
		{Name: "Drift Status", Weight: 10, Score: driftScore},
	}
	overall := 0.0
	for _, c := range categories {
		overall += c.Score
	}
	if overall > 100 {
		overall = 100
	}
	return healthReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Repository:  inv.Repository,
		Overall:     overall,
		Categories:  categories,
	}
}

func writeWorkflowFiles(repoRoot string, artifacts *scanner.Artifacts) ([]string, error) {
	workflowDir := filepath.Join(repoRoot, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		return nil, err
	}
	files := map[string]string{
		"api-smoke.yml":       workflowYAML("api-smoke", "node "+filepath.ToSlash(artifacts.HarnessPath)),
		"api-contract.yml":    workflowYAML("api-contract", "test -f "+filepath.ToSlash(artifacts.OpenAPIPath)),
		"api-regression.yml":  workflowYAML("api-regression", "node "+filepath.ToSlash(filepath.Join(filepath.Dir(artifacts.HarnessPath), "scan.runner.js"))),
		"api-security.yml":    workflowYAML("api-security", "grep -R \"authorization\\|apiKey\\|oauth\" "+filepath.ToSlash(artifacts.OpenAPIPath)+" || true"),
		"schema-drift.yml":    workflowYAML("schema-drift", "cat "+filepath.ToSlash(artifacts.DriftPath)),
		"sdk-generation.yml":  workflowYAML("sdk-generation", "echo \"run reqit sdk generate .\""),
	}
	paths := make([]string, 0, len(files))
	for name, content := range files {
		p := filepath.Join(workflowDir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, nil
}

func workflowYAML(name, command string) string {
	return fmt.Sprintf(`name: %s
on:
  push:
    branches: [ main, master ]
  pull_request:
    branches: [ main, master ]
  schedule:
    - cron: "0 2 * * *"
jobs:
  run:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Execute
        run: %s
`, name, command)
}

func writeSDKFiles(dir string, inv *scanner.Inventory) error {
	manifest := map[string]any{
		"generatedAt": time.Now().UTC().Format(time.RFC3339),
		"languages":   []string{"typescript", "javascript", "python", "rust", "go", "java", "kotlin", "swift", "csharp"},
		"endpoints":   len(inv.Endpoints),
	}
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), manifestBytes, 0o644); err != nil {
		return err
	}
	ts := "export async function request(method: string, url: string, body?: unknown) {\n  return fetch(url, { method, headers: { 'content-type': 'application/json' }, body: body ? JSON.stringify(body) : undefined });\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "typescript.ts"), []byte(ts), 0o644); err != nil {
		return err
	}
	py := "import requests\n\ndef request(method, url, body=None):\n    return requests.request(method, url, json=body)\n"
	if err := os.WriteFile(filepath.Join(dir, "python.py"), []byte(py), 0o644); err != nil {
		return err
	}
	goClient := "package sdk\n\nimport (\"bytes\"; \"net/http\")\n\nfunc Request(method, url string, body []byte) (*http.Response, error) {\n\treq, err := http.NewRequest(method, url, bytes.NewReader(body))\n\tif err != nil { return nil, err }\n\treq.Header.Set(\"Content-Type\", \"application/json\")\n\treturn http.DefaultClient.Do(req)\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "go.go"), []byte(goClient), 0o644); err != nil {
		return err
	}
	rustClient := "pub async fn request(method: reqwest::Method, url: &str, body: Option<serde_json::Value>) -> reqwest::Result<reqwest::Response> {\n    let client = reqwest::Client::new();\n    let mut req = client.request(method, url);\n    if let Some(b) = body { req = req.json(&b); }\n    req.send().await\n}\n"
	return os.WriteFile(filepath.Join(dir, "rust.rs"), []byte(rustClient), 0o644)
}

func listWatchDirs(root string) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		switch base {
		case ".git", "node_modules", "vendor", "dist", "build", ".next", ".turbo":
			return filepath.SkipDir
		}
		dirs = append(dirs, path)
		return nil
	})
	return dirs, err
}

func githubTokenPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".reqit")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "github.token"), nil
}

func saveGitHubToken(token string) error {
	p, err := githubTokenPath()
	if err != nil {
		return err
	}
	return os.WriteFile(p, []byte(strings.TrimSpace(token)), 0o600)
}

func loadGitHubToken() (string, error) {
	if t := strings.TrimSpace(os.Getenv("REQIT_GITHUB_TOKEN")); t != "" {
		return t, nil
	}
	p, err := githubTokenPath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(b))
	if token == "" {
		return "", errors.New("empty token")
	}
	return token, nil
}

func gitExec(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func parseRepoSlugFromRemote(repoDir string) (string, string, error) {
	remote, err := gitExec(repoDir, "remote", "get-url", "origin")
	if err != nil {
		return "", "", err
	}
	s := strings.TrimSpace(remote)
	s = strings.TrimSuffix(s, ".git")
	if strings.HasPrefix(s, "git@github.com:") {
		s = strings.TrimPrefix(s, "git@github.com:")
	} else if strings.Contains(s, "github.com/") {
		s = s[strings.Index(s, "github.com/")+len("github.com/"):]
	}
	parts := strings.Split(strings.Trim(s, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("unsupported remote URL: %s", remote)
	}
	return parts[len(parts)-2], parts[len(parts)-1], nil
}
