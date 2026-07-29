package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	gogit "github.com/go-git/go-git/v5"

	gitpkg "flux/internal/git"
	"flux/internal/scanner"
)

type RepositoryService interface {
	Clone(url string) error
	Open(path string) error
	Scan() (*scanner.Inventory, error)
	SwitchBranch(name string) error
	DetectChanges() (bool, error)
	Workspace() *Manager
}

type LocalRepositoryService struct {
	manager *Manager
	root    string
	git     *gitpkg.Store
}

func NewRepositoryService(manager *Manager) *LocalRepositoryService {
	return &LocalRepositoryService{manager: manager}
}

func (s *LocalRepositoryService) Workspace() *Manager {
	return s.manager
}

func (s *LocalRepositoryService) Root() string {
	return s.root
}

func (s *LocalRepositoryService) Clone(url string) error {
	if s.manager == nil {
		return fmt.Errorf("workspace manager is required")
	}
	if err := s.manager.Ensure(); err != nil {
		return err
	}
	dir := filepath.Join(s.manager.Root(), deriveRepoDirName(url))
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("repository already exists at %s", dir)
	}
	if _, err := gogit.PlainClone(dir, false, &gogit.CloneOptions{URL: url}); err != nil {
		return err
	}
	return s.Open(dir)
}

func (s *LocalRepositoryService) Open(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	stat, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if !stat.IsDir() {
		return fmt.Errorf("repository path must be a directory")
	}
	g, err := gitpkg.Open(abs)
	if err != nil {
		return err
	}
	s.root = abs
	s.git = g
	return nil
}

func (s *LocalRepositoryService) Scan() (*scanner.Inventory, error) {
	if s.root == "" {
		return nil, fmt.Errorf("repository is not opened")
	}
	return scanner.ScanRepository(s.root)
}

func (s *LocalRepositoryService) SwitchBranch(name string) error {
	if s.git == nil {
		return fmt.Errorf("repository is not opened")
	}
	return s.git.SwitchBranch(name)
}

func (s *LocalRepositoryService) DetectChanges() (bool, error) {
	if s.git == nil {
		return false, fmt.Errorf("repository is not opened")
	}
	return s.git.HasChanges(), nil
}

func deriveRepoDirName(url string) string {
	name := strings.TrimSpace(url)
	name = strings.TrimSuffix(name, ".git")
	if idx := strings.LastIndexAny(name, "/:"); idx >= 0 && idx+1 < len(name) {
		name = name[idx+1:]
	}
	name = strings.TrimSpace(name)
	if name == "" || name == "." {
		return fmt.Sprintf("repository-%d", time.Now().UTC().UnixNano())
	}
	return name
}
