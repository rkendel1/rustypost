import { create } from "zustand";
import { ListSecrets, StoreSecret, DeleteSecret, RotateSecret } from "../../../wailsjs/go/main/App";
import type { vault } from "../../../wailsjs/go/models";
import { toast } from "@/app/stores/useToastStore";

// useVaultStore only ever holds SecretMetadata — never a secret value. There
// is no action here that can retrieve one; StoreSecret/RotateSecret only
// ever send a value TO the backend, they don't return it.
type VaultStore = {
  secrets: vault.SecretMetadata[];
  loaded: boolean;
  loadError: string | null;

  load: (projectID?: string) => Promise<void>;
  store: (input: vault.StoreSecretInput) => Promise<vault.SecretReference>;
  remove: (secretID: string) => Promise<void>;
  rotate: (secretID: string, newValue: string) => Promise<void>;
};

export const useVaultStore = create<VaultStore>((set, get) => ({
  secrets: [],
  loaded: false,
  loadError: null,

  load: async (projectID = "") => {
    try {
      const secrets = await ListSecrets(projectID);
      set({ secrets, loaded: true, loadError: null });
    } catch (e) {
      const msg = e instanceof Error ? e.message : "Failed to load secrets";
      toast.error(msg);
      set({ loaded: true, loadError: msg });
    }
  },

  store: async (input) => {
    const ref = await StoreSecret(input);
    await get().load();
    return ref;
  },

  remove: async (secretID) => {
    await DeleteSecret(secretID);
    await get().load();
  },

  rotate: async (secretID, newValue) => {
    await RotateSecret(secretID, newValue);
    await get().load();
  },
}));
