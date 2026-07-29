package workspace

import (
	"os"
	"path/filepath"
)

type ArtifactLayout struct {
	ReqitRoot       string `json:"reqitRoot"`
	CollectionsDir  string `json:"collectionsDir"`
	EnvironmentsDir string `json:"environmentsDir"`
	ReportsDir      string `json:"reportsDir"`
	OpenAPIDir      string `json:"openapiDir"`
	TestsDir        string `json:"testsDir"`
	WorkflowsDir    string `json:"workflowsDir"`
}

func TargetArtifactLayout(repositoryRoot string) ArtifactLayout {
	reqitRoot := filepath.Join(repositoryRoot, ".reqit")
	return ArtifactLayout{
		ReqitRoot:       reqitRoot,
		CollectionsDir:  filepath.Join(reqitRoot, "collections"),
		EnvironmentsDir: filepath.Join(reqitRoot, "environments"),
		ReportsDir:      filepath.Join(reqitRoot, "reports"),
		OpenAPIDir:      filepath.Join(repositoryRoot, "openapi"),
		TestsDir:        filepath.Join(repositoryRoot, "tests"),
		WorkflowsDir:    filepath.Join(repositoryRoot, ".github", "workflows"),
	}
}

func EnsureTargetArtifactLayout(repositoryRoot string) (ArtifactLayout, error) {
	layout := TargetArtifactLayout(repositoryRoot)
	dirs := []string{
		layout.CollectionsDir,
		layout.EnvironmentsDir,
		layout.ReportsDir,
		layout.OpenAPIDir,
		layout.TestsDir,
		layout.WorkflowsDir,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return ArtifactLayout{}, err
		}
	}
	return layout, nil
}
