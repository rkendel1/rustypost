import { useEffect, useState } from "react";
import { useUIStore } from "@/app/stores/useUIStore";
import { useProjectStore } from "../stores/useProjectStore";
import { useCapabilities } from "@/capabilities/useCapabilities";
import { PickFolder } from "../../../wailsjs/go/main/App";
import { toast } from "@/app/stores/useToastStore";

const AVAILABILITY_LABEL: Record<string, string> = {
  available: "Available",
  unavailable: "Unavailable",
  coming_soon: "Coming Soon",
  requires_setup: "Requires Setup",
};

// ProjectsPanel is the in-app Project view: the active project's sources
// and registered capabilities (internal/capabilities), backed by real
// bindings — no fake/stub data.
export function ProjectsPanel() {
  const setView = useUIStore((s) => s.setView);
  const activeID = useProjectStore((s) => s.activeID);
  const active = useProjectStore((s) => s.activeProject);
  const load = useProjectStore((s) => s.load);
  const addSource = useProjectStore((s) => s.addSource);
  const removeSource = useProjectStore((s) => s.removeSource);

  const caps = useCapabilities(activeID);

  const [sourcePath, setSourcePath] = useState("");
  const [sourceName, setSourceName] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    void load();
  }, [load]);

  const pickSourceFolder = async () => {
    try {
      const dir = await PickFolder("Choose a source folder");
      if (dir) setSourcePath(dir);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to pick a folder");
    }
  };

  const handleAddSource = async () => {
    if (!activeID || !sourcePath.trim()) {
      toast.error("Choose a folder first.");
      return;
    }
    setBusy(true);
    try {
      await addSource(activeID, sourceName.trim() || sourcePath.trim(), "local_folder", ".", "", "");
      setSourcePath("");
      setSourceName("");
      toast.success("Source added.");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to add source");
    } finally {
      setBusy(false);
    }
  };

  const handleRemoveSource = async (sourceID: string) => {
    if (!activeID) return;
    setBusy(true);
    try {
      await removeSource(activeID, sourceID);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to remove source");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex-1 flex flex-col min-w-0 bg-bg">
      <header className="h-[48px] flex items-center gap-3 px-4 border-b border-border shrink-0">
        <button onClick={() => setView("http")} className="text-subtext hover:text-text text-13">&larr; Back</button>
        <h1 className="text-14 font-semibold text-text">{active?.name ?? "Project"}</h1>
      </header>
      <div className="flex-1 overflow-y-auto p-4 space-y-5 max-w-3xl">
        <section className="rounded-xl border border-border bg-surface p-4 space-y-3">
          <h2 className="text-14 font-semibold text-text">Sources</h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-2 items-end">
            <div className="md:col-span-2">
              <label className="text-12 text-subtext block mb-1">Folder</label>
              <div className="flex gap-2">
                <input
                  value={sourcePath}
                  onChange={(e) => setSourcePath(e.target.value)}
                  placeholder="/path/to/repo"
                  className="flex-1 px-3 py-1.5 rounded-lg bg-bg border border-border text-13 text-text"
                />
                <button type="button" onClick={pickSourceFolder} className="px-3 py-1.5 rounded-lg border border-border text-13 text-text hover:bg-cardHover transition-colors">
                  Browse
                </button>
              </div>
            </div>
            <button
              type="button"
              disabled={busy}
              onClick={handleAddSource}
              className="h-[34px] rounded-lg bg-cyan/10 text-cyan text-13 font-semibold hover:bg-cyan/15 transition-colors disabled:opacity-50"
            >
              Add Source
            </button>
          </div>
          <p className="text-11 text-subtext">
            Reqit detects whether a folder is a git repository or a plain local folder automatically.
          </p>
          {active && active.sources.length > 0 && (
            <div className="space-y-1.5 pt-1">
              {active.sources.map((src) => (
                <div key={src.id} className="flex items-center justify-between gap-2 px-3 py-2 rounded-lg border border-border">
                  <div className="min-w-0">
                    <div className="text-13 font-medium text-text truncate">{src.name}</div>
                    <div className="text-11 text-subtext truncate">{src.type}{src.path ? ` · ${src.path}` : ""}</div>
                  </div>
                  <button
                    type="button"
                    disabled={busy}
                    onClick={() => handleRemoveSource(src.id)}
                    className="text-11 text-subtext hover:text-red-500 transition-colors shrink-0"
                  >
                    Remove
                  </button>
                </div>
              ))}
            </div>
          )}
        </section>

        <section className="rounded-xl border border-border bg-surface p-4 space-y-3">
          <h2 className="text-14 font-semibold text-text">Capabilities</h2>
          {caps.length === 0 ? (
            <div className="text-13 text-subtext">No capabilities registered yet.</div>
          ) : (
            <div className="space-y-1.5">
              {caps.map((c) => (
                <div key={c.id} className="flex items-center justify-between gap-2 px-3 py-2 rounded-lg border border-border">
                  <div className="min-w-0">
                    <div className="text-13 font-medium text-text">{c.name}</div>
                    <div className="text-11 text-subtext truncate">{c.description}</div>
                  </div>
                  <span
                    className={
                      "text-10 font-semibold px-2 py-0.5 rounded-full shrink-0 " +
                      (c.availability === "available"
                        ? "bg-emerald-500/10 text-emerald-500"
                        : c.availability === "requires_setup"
                          ? "bg-amber-500/10 text-amber-500"
                          : "bg-subtext/10 text-subtext")
                    }
                  >
                    {AVAILABILITY_LABEL[c.availability] ?? c.availability}
                  </span>
                </div>
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
