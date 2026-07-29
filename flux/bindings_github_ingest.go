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
)

func (a *App) GitHubSavePAT(account, token string) error {
	auth := githubsvc.NewAuthService(account)
	return auth.SaveToken(token)
}

func (a *App) GitHubDeletePAT(account string) error {
	auth := githubsvc.NewAuthService(account)
	return auth.DeleteToken()
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

func (a *App) RunRepoAutomation(command, repoPath, outputDir string) (string, error) {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return "", fmt.Errorf("command is required")
	}
	absRepo, resolvedOutput, err := resolveRepoAndOutput(repoPath, outputDir)
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

func resolveRepoAndOutput(repoPath, outputDir string) (string, string, error) {
	absRepo, err := filepath.Abs(strings.TrimSpace(repoPath))
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
		return absRepo, filepath.Join(absRepo, ".reqit", "scan"), nil
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