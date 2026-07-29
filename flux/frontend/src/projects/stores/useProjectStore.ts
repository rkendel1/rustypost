import { create } from "zustand";
import {
  ListProjects,
  CreateProject,
  OpenProject,
  GetActiveProject,
  UpdateProject,
  ArchiveProject,
  AddProjectSource,
  RemoveProjectSource,
} from "../../../wailsjs/go/main/App";
import type { projects } from "../../../wailsjs/go/models";
import { toast } from "@/app/stores/useToastStore";
import { useCollectionStore } from "@/features/collections/stores/useCollectionStore";
import { useHistoryStore } from "@/features/history/stores/useHistoryStore";
import { useEnvStore } from "@/features/env/stores/useEnvStore";

type ProjectStore = {
  projects: projects.ProjectSummary[];
  activeID: string | null;
  activeProject: projects.Project | null;
  loaded: boolean;
  loadError: string | null;

  load: () => Promise<void>;
  create: (name: string, description: string, rootDir: string) => Promise<projects.Project>;
  open: (id: string) => Promise<void>;
  update: (id: string, name?: string, description?: string) => Promise<projects.Project>;
  archive: (id: string) => Promise<void>;
  addSource: (
    projectID: string,
    name: string,
    kind: string,
    path: string,
    url: string,
    branch: string,
  ) => Promise<projects.ProjectSource>;
  removeSource: (projectID: string, sourceID: string) => Promise<void>;
};

export const useProjectStore = create<ProjectStore>((set, get) => ({
  projects: [],
  activeID: null,
  activeProject: null,
  loaded: false,
  loadError: null,

  load: async () => {
    try {
      const [all, active] = await Promise.all([ListProjects(), GetActiveProject()]);
      set({
        projects: all,
        activeID: active?.id ?? null,
        activeProject: active ?? null,
        loaded: true,
        loadError: null,
      });
    } catch (e) {
      const msg = e instanceof Error ? e.message : "Failed to load projects";
      toast.error(msg);
      set({ loaded: true, loadError: msg });
    }
  },

  create: async (name, description, rootDir) => {
    const project = await CreateProject(name, description, rootDir);
    await get().load();
    return project;
  },

  open: async (id) => {
    await OpenProject(id);
    await get().load();
    await Promise.all([
      useCollectionStore.getState().load(),
      useHistoryStore.getState().load(),
      useEnvStore.getState().load(),
    ]);
  },

  update: async (id, name, description) => {
    const project = await UpdateProject(id, name ?? null, description ?? null);
    await get().load();
    return project;
  },

  archive: async (id) => {
    await ArchiveProject(id);
    await get().load();
  },

  addSource: async (projectID, name, kind, path, url, branch) => {
    const source = await AddProjectSource(projectID, name, kind as any, path, url, branch);
    await get().load();
    return source;
  },

  removeSource: async (projectID, sourceID) => {
    await RemoveProjectSource(projectID, sourceID);
    await get().load();
  },
}));
