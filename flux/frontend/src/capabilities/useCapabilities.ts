import { useEffect, useState } from "react";
import { GetProjectCapabilities } from "../../wailsjs/go/main/App";
import type { capabilities } from "../../wailsjs/go/models";

// useCapabilities queries which capabilities are available for a project
// (see internal/capabilities.Registry) so UI can render/gate features from
// real backend state instead of a hardcoded list. Returns an empty array
// until projectID resolves and the query completes.
export function useCapabilities(projectID: string | null): capabilities.Snapshot[] {
  const [snapshot, setSnapshot] = useState<capabilities.Snapshot[]>([]);

  useEffect(() => {
    if (!projectID) {
      setSnapshot([]);
      return;
    }
    let cancelled = false;
    GetProjectCapabilities(projectID)
      .then((snap) => {
        if (!cancelled) setSnapshot(snap);
      })
      .catch(() => {
        if (!cancelled) setSnapshot([]);
      });
    return () => {
      cancelled = true;
    };
  }, [projectID]);

  return snapshot;
}
