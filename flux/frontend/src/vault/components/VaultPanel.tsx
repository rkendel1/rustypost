import { useEffect, useState } from "react";
import { useUIStore } from "@/app/stores/useUIStore";
import { useVaultStore } from "../stores/useVaultStore";
import { toast } from "@/app/stores/useToastStore";
import { showConfirm } from "@/shared/components/ConfirmModal";

const SECRET_TYPES = ["api_key", "access_token", "refresh_token", "password", "private_key", "connection", "custom"];
const SECRET_SCOPES = ["application", "project", "environment"];

// VaultPanel is the Secrets Vault UI: metadata only, backed by
// ListSecrets/StoreSecret/DeleteSecret/RotateSecret. There is no "reveal"
// affordance anywhere in this component, and none is possible — the
// underlying bindings never return a secret value once stored.
export function VaultPanel() {
  const setView = useUIStore((s) => s.setView);
  const load = useVaultStore((s) => s.load);
  const secrets = useVaultStore((s) => s.secrets);
  const loaded = useVaultStore((s) => s.loaded);
  const store = useVaultStore((s) => s.store);
  const remove = useVaultStore((s) => s.remove);

  const [adding, setAdding] = useState(false);
  const [name, setName] = useState("");
  const [type, setType] = useState(SECRET_TYPES[0]);
  const [scope, setScope] = useState(SECRET_SCOPES[0]);
  const [value, setValue] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    void load();
  }, [load]);

  const handleSave = async () => {
    if (!name.trim() || !value.trim()) {
      toast.error("Name and value are required.");
      return;
    }
    setBusy(true);
    try {
      await store({
        Name: name.trim(),
        Description: "",
        Type: type,
        Scope: scope,
        Value: value,
        ProjectID: "",
        Provider: "",
      } as any);
      // Clear the entered value from frontend state immediately after save.
      setValue("");
      setName("");
      setAdding(false);
      toast.success("Secret stored securely.");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to store secret");
    } finally {
      setBusy(false);
    }
  };

  const handleDelete = async (secretID: string, secretName: string) => {
    const confirmed = await showConfirm({
      title: "Delete secret",
      message: `Delete "${secretName}"? Anything still referencing it will stop working.`,
      confirmLabel: "Delete",
      variant: "danger",
    });
    if (!confirmed) return;
    try {
      await remove(secretID);
      toast.success("Secret deleted.");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to delete secret");
    }
  };

  return (
    <div className="flex-1 flex flex-col min-w-0 bg-bg">
      <header className="h-[48px] flex items-center gap-3 px-4 border-b border-border shrink-0">
        <button onClick={() => setView("http")} className="text-subtext hover:text-text text-13">&larr; Back</button>
        <h1 className="text-14 font-semibold text-text">Vault</h1>
      </header>
      <div className="flex-1 overflow-y-auto p-4 space-y-4 max-w-2xl">
        {!loaded ? (
          <div className="text-13 text-subtext">Loading…</div>
        ) : secrets.length === 0 ? (
          <div className="text-13 text-subtext">No secrets stored yet.</div>
        ) : (
          <div className="space-y-1.5">
            {secrets.map((s) => (
              <div key={s.id} className="flex items-center justify-between gap-2 px-3 py-2.5 rounded-lg border border-border bg-surface">
                <div className="min-w-0">
                  <div className="text-13 font-medium text-text truncate">{s.name}</div>
                  <div className="text-11 text-subtext truncate">
                    {s.type} · {s.provider} · {s.status}
                    {s.updatedAt ? ` · updated ${new Date(s.updatedAt).toLocaleDateString()}` : ""}
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => handleDelete(s.id, s.name)}
                  className="text-11 text-subtext hover:text-red-500 transition-colors shrink-0"
                >
                  Delete
                </button>
              </div>
            ))}
          </div>
        )}

        {!adding ? (
          <button
            type="button"
            onClick={() => setAdding(true)}
            className="w-full h-[36px] rounded-lg bg-cyan/10 text-cyan text-13 font-semibold hover:bg-cyan/15 transition-colors"
          >
            + Add Secret
          </button>
        ) : (
          <div className="rounded-xl border border-border bg-surface p-4 space-y-3">
            <h2 className="text-14 font-semibold text-text">Add Secret</h2>
            <div>
              <label className="text-12 text-subtext block mb-1">Name</label>
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="GitHub Account"
                className="w-full px-3 py-1.5 rounded-lg bg-bg border border-border text-13 text-text"
              />
            </div>
            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className="text-12 text-subtext block mb-1">Type</label>
                <select value={type} onChange={(e) => setType(e.target.value)} className="w-full px-3 py-1.5 rounded-lg bg-bg border border-border text-13 text-text">
                  {SECRET_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
                </select>
              </div>
              <div>
                <label className="text-12 text-subtext block mb-1">Scope</label>
                <select value={scope} onChange={(e) => setScope(e.target.value)} className="w-full px-3 py-1.5 rounded-lg bg-bg border border-border text-13 text-text">
                  {SECRET_SCOPES.map((s) => <option key={s} value={s}>{s}</option>)}
                </select>
              </div>
            </div>
            <div>
              <label className="text-12 text-subtext block mb-1">Value</label>
              <input
                value={value}
                onChange={(e) => setValue(e.target.value)}
                type="password"
                placeholder="••••••••••••••"
                className="w-full px-3 py-1.5 rounded-lg bg-bg border border-border text-13 text-text"
              />
            </div>
            <div className="flex gap-2">
              <button
                type="button"
                disabled={busy}
                onClick={handleSave}
                className="flex-1 h-[34px] rounded-lg bg-cyan text-bg text-13 font-semibold hover:opacity-90 transition-opacity disabled:opacity-50"
              >
                Save Securely
              </button>
              <button
                type="button"
                onClick={() => { setAdding(false); setValue(""); }}
                className="px-4 h-[34px] rounded-lg border border-border text-13 text-text hover:bg-cardHover transition-colors"
              >
                Cancel
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
