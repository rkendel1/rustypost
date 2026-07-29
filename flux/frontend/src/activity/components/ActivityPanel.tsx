import { useEffect, useState } from "react";
import { useUIStore } from "@/app/stores/useUIStore";
import { GetProjectActivity } from "../../../wailsjs/go/main/App";
import type { history } from "../../../wailsjs/go/models";
import { toast } from "@/app/stores/useToastStore";

// ActivityPanel is the project timeline: masker-redacted job/pipeline
// history (internal/automation/history), most recent first.
export function ActivityPanel() {
  const setView = useUIStore((s) => s.setView);
  const [entries, setEntries] = useState<history.Entry[]>([]);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    GetProjectActivity(100)
      .then((rows) => {
        setEntries([...rows].reverse());
        setLoaded(true);
      })
      .catch((e) => {
        toast.error(e instanceof Error ? e.message : "Failed to load activity");
        setLoaded(true);
      });
  }, []);

  return (
    <div className="flex-1 flex flex-col min-w-0 bg-bg">
      <header className="h-[48px] flex items-center gap-3 px-4 border-b border-border shrink-0">
        <button onClick={() => setView("http")} className="text-subtext hover:text-text text-13">&larr; Back</button>
        <h1 className="text-14 font-semibold text-text">Activity</h1>
      </header>
      <div className="flex-1 overflow-y-auto p-4 max-w-2xl">
        {!loaded ? (
          <div className="text-13 text-subtext">Loading…</div>
        ) : entries.length === 0 ? (
          <div className="text-13 text-subtext">No activity yet for this project.</div>
        ) : (
          <div className="space-y-1.5">
            {entries.map((e) => (
              <div key={e.id} className="px-3 py-2.5 rounded-lg border border-border bg-surface">
                <div className="flex items-center justify-between gap-2">
                  <span className="text-13 font-medium text-text">{e.name || e.kind}</span>
                  <span
                    className={
                      "text-10 font-semibold px-2 py-0.5 rounded-full shrink-0 " +
                      (e.status === "completed"
                        ? "bg-emerald-500/10 text-emerald-500"
                        : e.status === "failed"
                          ? "bg-red-500/10 text-red-500"
                          : "bg-subtext/10 text-subtext")
                    }
                  >
                    {e.status}
                  </span>
                </div>
                {e.finishedAt && (
                  <div className="text-11 text-subtext mt-0.5">
                    {new Date(e.finishedAt).toLocaleString()}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
