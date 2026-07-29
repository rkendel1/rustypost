import { useEffect, useState } from "react";
import { useProjectStore } from "../stores/useProjectStore";
import { PickFolder } from "../../../wailsjs/go/main/App";
import { toast } from "@/app/stores/useToastStore";
import reqitLogo from "../../assets/images/reqitlogo.jpeg";

// ProjectPicker is the new "home" screen: it replaces the workspace picker
// (@/app/screens/HomeScreen + @/features/workspace) as the entry point,
// backed by the Project domain model (internal/projects) instead of
// Workspace. It's intentionally focused on the core create/list/open flow —
// HomeScreen's marketing content (GitHub stars, blog, etc.) is left for a
// follow-up to fold in once this flow has been used for real.
export function ProjectPicker({ onEnter }: { onEnter: () => void }) {
  const load = useProjectStore((s) => s.load);
  const projectList = useProjectStore((s) => s.projects);
  const loaded = useProjectStore((s) => s.loaded);
  const open = useProjectStore((s) => s.open);
  const create = useProjectStore((s) => s.create);

  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [rootDir, setRootDir] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    void load();
  }, [load]);

  const pickFolder = async () => {
    try {
      const dir = await PickFolder("Choose a folder for this project");
      if (dir) setRootDir(dir);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to pick a folder");
    }
  };

  const handleCreate = async () => {
    if (!name.trim() || !rootDir.trim()) {
      toast.error("Name and folder are required.");
      return;
    }
    setBusy(true);
    try {
      const project = await create(name.trim(), description.trim(), rootDir.trim());
      await open(project.id);
      onEnter();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to create project");
    } finally {
      setBusy(false);
    }
  };

  const handleOpen = async (id: string) => {
    setBusy(true);
    try {
      await open(id);
      onEnter();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to open project");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="h-screen w-screen bg-bg flex items-center justify-center p-8">
      <div className="w-full max-w-2xl space-y-6">
        <div className="flex items-center gap-3">
          <img src={reqitLogo} alt="reqit" className="h-[36px] w-auto object-contain" />
          <h1 className="text-18 font-bold text-text">Projects</h1>
        </div>

        <div className="rounded-xl border border-border bg-surface p-4 space-y-3">
          <h2 className="text-14 font-semibold text-text">
            {creating ? "New Project" : "Your Projects"}
          </h2>

          {!creating && (
            <>
              {!loaded ? (
                <div className="text-13 text-subtext">Loading…</div>
              ) : projectList.length === 0 ? (
                <div className="text-13 text-subtext">No projects yet — create one to get started.</div>
              ) : (
                <div className="space-y-1.5 max-h-[320px] overflow-y-auto">
                  {projectList.map((p) => (
                    <button
                      key={p.id}
                      type="button"
                      disabled={busy}
                      onClick={() => handleOpen(p.id)}
                      className="w-full text-left px-3 py-2 rounded-lg border border-border hover:bg-cardHover transition-colors flex items-center justify-between gap-2"
                    >
                      <div className="min-w-0">
                        <div className="text-13 font-medium text-text truncate">{p.name}</div>
                        <div className="text-11 text-subtext truncate">{p.rootDir}</div>
                      </div>
                      <span className="text-10 text-subtext shrink-0">{p.sourceCount} source{p.sourceCount === 1 ? "" : "s"}</span>
                    </button>
                  ))}
                </div>
              )}
              <button
                type="button"
                onClick={() => setCreating(true)}
                className="w-full h-[36px] rounded-lg bg-cyan/10 text-cyan text-13 font-semibold hover:bg-cyan/15 transition-colors"
              >
                + New Project
              </button>
            </>
          )}

          {creating && (
            <div className="space-y-3">
              <div>
                <label className="text-12 text-subtext block mb-1">Name</label>
                <input
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="FieldFlow"
                  className="w-full px-3 py-1.5 rounded-lg bg-bg border border-border text-13 text-text"
                />
              </div>
              <div>
                <label className="text-12 text-subtext block mb-1">Description (optional)</label>
                <input
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="Field service operations platform"
                  className="w-full px-3 py-1.5 rounded-lg bg-bg border border-border text-13 text-text"
                />
              </div>
              <div>
                <label className="text-12 text-subtext block mb-1">Project folder</label>
                <div className="flex gap-2">
                  <input
                    value={rootDir}
                    onChange={(e) => setRootDir(e.target.value)}
                    placeholder="/Users/you/dev/fieldflow"
                    className="flex-1 px-3 py-1.5 rounded-lg bg-bg border border-border text-13 text-text"
                  />
                  <button
                    type="button"
                    onClick={pickFolder}
                    className="px-3 py-1.5 rounded-lg border border-border text-13 text-text hover:bg-cardHover transition-colors"
                  >
                    Browse
                  </button>
                </div>
              </div>
              <div className="flex gap-2 pt-1">
                <button
                  type="button"
                  disabled={busy}
                  onClick={handleCreate}
                  className="flex-1 h-[36px] rounded-lg bg-cyan text-bg text-13 font-semibold hover:opacity-90 transition-opacity disabled:opacity-50"
                >
                  Create Project
                </button>
                <button
                  type="button"
                  onClick={() => setCreating(false)}
                  className="px-4 h-[36px] rounded-lg border border-border text-13 text-text hover:bg-cardHover transition-colors"
                >
                  Cancel
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
