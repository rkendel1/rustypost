package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"flux/internal/cli"
	githubsvc "flux/internal/github"
	"flux/internal/integrations"
	"flux/internal/projects"
)

func (a *App) GitHubSavePAT(account, token string) error {
	auth := githubsvc.NewAuthService(account)
	if err := auth.SaveToken(token); err != nil {
		return err
	}
	// Additive, non-blocking: register this PAT as a vault secret +
	// Integration too, so future capabilities (which resolve credentials
	// through vault.SecretResolver, never through AuthService directly) see
	// it immediately rather than waiting for the next app restart.
	if a.vaultSvc != nil && a.integrations != nil {
		_ = integrations.MigrateGitHubPAT(context.Background(), account, a.vaultSvc, a.integrations)
	}
	return nil
}

func (a *App) GitHubDeletePAT(account string) error {
	auth := githubsvc.NewAuthService(account)
	if err := auth.DeleteToken(); err != nil {
		return err
	}
	// Remove the corresponding Integration record, but deliberately leave
	// the vault secret itself alone — it may be an application-scoped
	// credential shared elsewhere, and secret deletion is always its own
	// explicit, confirmed action (see VaultService.DeleteSecret).
	if a.integrations != nil {
		account = normalizeAccount(account)
		name := "GitHub (" + account + ")"
		if list, err := a.integrations.List(context.Background(), ""); err == nil {
			for _, in := range list {
				if in.Provider == integrations.ProviderGitHub && in.Name == name {
					_ = a.integrations.Remove(context.Background(), in.ID)
				}
			}
		}
	}
	return nil
}

func normalizeAccount(account string) string {
	account = strings.TrimSpace(account)
	if account == "" {
		account = "default"
	}
	return account
}

func (a *App) GitHubGetViewer(account string) (string, error) {
	auth := githubsvc.NewAuthService(account)
	client := githubsvc.NewClient(auth)
	viewer, err := client.GetViewer(context.Background())
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(viewer)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (a *App) GitHubListRepositories(account, visibility string) (string, error) {
	auth := githubsvc.NewAuthService(account)
	client := githubsvc.NewClient(auth)
	repos, err := client.ListRepositories(context.Background(), strings.TrimSpace(visibility))
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(repos)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (a *App) GitHubCloneRepository(account, repository, destinationDir string) (string, error) {
	repo := strings.TrimSpace(repository)
	if repo == "" {
		return "", fmt.Errorf("repository is required")
	}
	dest := strings.TrimSpace(destinationDir)
	if dest == "" {
		return "", fmt.Errorf("destination directory is required")
	}
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(absDest, 0o755); err != nil {
		return "", err
	}

	cloneURL, repoName := normalizeCloneInput(repo)
	target := filepath.Join(absDest, repoName)
	if _, err := os.Stat(target); err == nil {
		return "", fmt.Errorf("target already exists: %s", target)
	}

	authService := githubsvc.NewAuthService(account)
	token, _ := authService.LoadToken()
	cloneOpts := &gogit.CloneOptions{URL: cloneURL}
	if token != "" && strings.HasPrefix(strings.ToLower(cloneURL), "https://") {
		cloneOpts.Auth = &githttp.BasicAuth{Username: "x-access-token", Password: token}
	}

	if _, err := gogit.PlainClone(target, false, cloneOpts); err != nil {
		return "", err
	}
	return target, nil
}

// RunRepoAutomation runs a scan/generate/health/drift/report/etc. command
// against a Project Source rather than a bare repository path — the repo
// path and output directory are both resolved through the Project model
// (internal/projects), replacing the old hardcoded <repo>/.reqit/scan
// default with the project's own local-state cache directory. outputDir
// may still be supplied explicitly (absolute, or relative to the resolved
// repo path) to override that default.
func (a *App) RunRepoAutomation(command, projectID, sourceID, outputDir string) (string, error) {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return "", fmt.Errorf("command is required")
	}
	absRepo, resolvedOutput, err := a.resolveProjectSourceAndOutput(projectID, sourceID, outputDir)
	if err != nil {
		return "", err
	}

	run := func(args []string) error {
		code := cli.Run(args)
		if code != 0 {
			return fmt.Errorf("command failed: %s", strings.Join(args, " "))
		}
		return nil
	}

	withOutput := func(args []string) []string {
		if strings.TrimSpace(outputDir) != "" {
			return append(args, "--output", resolvedOutput)
		}
		return args
	}

	switch cmd {
	case "scan":
		err = run(withOutput([]string{"scan", absRepo}))
	case "generate":
		err = run(withOutput([]string{"generate", absRepo}))
	case "health":
		err = run(withOutput([]string{"health", absRepo}))
	case "drift":
		err = run(withOutput([]string{"drift", absRepo}))
	case "report":
		err = run(withOutput([]string{"report", absRepo}))
	case "workflow-install":
		err = run(withOutput([]string{"workflow", "install", absRepo}))
	case "sdk-generate":
		err = run(withOutput([]string{"sdk", "generate", absRepo}))
	case "full-setup":
		steps := [][]string{
			withOutput([]string{"scan", absRepo}),
			withOutput([]string{"generate", absRepo}),
			withOutput([]string{"workflow", "install", absRepo}),
			withOutput([]string{"sdk", "generate", absRepo}),
			withOutput([]string{"health", absRepo}),
			withOutput([]string{"report", absRepo}),
		}
		for _, step := range steps {
			if runErr := run(step); runErr != nil {
				err = runErr
				break
			}
		}
	default:
		return "", fmt.Errorf("unsupported command: %s", cmd)
	}
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Command '%s' completed. Output: %s", cmd, resolvedOutput), nil
}

// resolveProjectSourceAndOutput resolves a Project + Source pair to a
// concrete repository path, and an output directory that defaults to the
// project's own .reqit/cache/scan local-state directory rather than a
// hardcoded path under the repo itself.
func (a *App) resolveProjectSourceAndOutput(projectID, sourceID, outputDir string) (string, string, error) {
	if a.projects == nil {
		return "", "", fmt.Errorf("projects not initialised")
	}
	project, err := a.projects.Open(a.ctx, projectID)
	if err != nil {
		return "", "", err
	}
	var src *projects.ProjectSource
	for i := range project.Sources {
		if project.Sources[i].ID == sourceID {
			src = &project.Sources[i]
			break
		}
	}
	if src == nil {
		return "", "", fmt.Errorf("source %q not found on project %q", sourceID, projectID)
	}
	absRepo, err := projects.ResolveSourcePath(project.RootDir, *src)
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(absRepo)
	if err != nil {
		return "", "", err
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("repository path must be a directory")
	}
	if strings.TrimSpace(outputDir) == "" {
		return absRepo, filepath.Join(projects.CacheDir(project.RootDir), "scan"), nil
	}
	if filepath.IsAbs(outputDir) {
		return absRepo, outputDir, nil
	}
	return absRepo, filepath.Join(absRepo, outputDir), nil
}

func normalizeCloneInput(input string) (cloneURL, name string) {
	v := strings.TrimSpace(input)
	if strings.Contains(v, "://") {
		cloneURL = v
	} else {
		slug := strings.TrimPrefix(strings.TrimPrefix(v, "github.com/"), "https://github.com/")
		slug = strings.TrimPrefix(slug, "http://github.com/")
		slug = strings.TrimPrefix(slug, "/")
		slug = strings.TrimSuffix(slug, ".git")
		cloneURL = "https://github.com/" + slug + ".git"
	}
	base := cloneURL
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	base = strings.TrimSuffix(base, ".git")
	if base == "" {
		base = "repository"
	}
	return cloneURL, base
}
