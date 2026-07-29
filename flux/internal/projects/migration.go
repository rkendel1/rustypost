package projects

import (
	"context"
	"errors"

	"flux/internal/workspaces"
)

// errUnsupportedServiceForMigration is returned if MigrateFromWorkspaces is
// called with a ProjectService implementation other than the one NewService
// returns — migration relies on createWithID, an implementation detail not
// part of the public interface.
var errUnsupportedServiceForMigration = errors.New("projects: migration requires the default ProjectService implementation")

// MigrateFromWorkspaces converts each legacy workspace into a Project that
// wraps the same on-disk data, non-destructively and idempotently: it never
// deletes or modifies workspace data, and it reuses each workspace's
// existing ID as the new project's ID, so re-running migration on every
// startup finds an existing project and no-ops rather than creating a
// duplicate. This mirrors workspaces.Store.Migrate()'s own idempotent,
// non-destructive pattern one level up.
func MigrateFromWorkspaces(svc ProjectService, store *workspaces.Store) error {
	concrete, ok := svc.(*service)
	if !ok {
		return errUnsupportedServiceForMigration
	}
	ctx := context.Background()

	infos, err := store.GetAll()
	if err != nil {
		return err
	}
	for _, info := range infos {
		if _, err := concrete.Open(ctx, info.ID); err == nil {
			continue // already migrated
		}
		p, err := concrete.createWithID(info.ID, CreateProjectInput{
			Name:        info.Name,
			Description: info.Description,
			RootDir:     info.DataDir,
		})
		if err != nil {
			return err
		}
		if _, err := concrete.AddSource(ctx, p.ID, AddSourceInput{
			Name: info.Name,
			Kind: DetectSourceKind(info.DataDir),
			Path: ".",
		}); err != nil {
			return err
		}
	}

	// Mirror the workspace store's active workspace onto the project
	// registry, so a startup bootstrapping "the active project" (see
	// ActiveProjectID) mounts the same one the user last had open.
	if activeID, _, err := store.GetActive(); err == nil && activeID != "" {
		_ = concrete.reg.touch(activeID)
	}
	return nil
}
