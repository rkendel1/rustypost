import { useState } from "react";
import { useUIStore } from "@/app/stores/useUIStore";
import { Button } from "@/shared/components/Button";
import {
  GetTelemetryConfig, SetTelemetryEnabled, GetTelemetryPreview, GetTelemetryEvents,
  PushToSwaggerHub, PullFromSwaggerHub, PushToStoplight, PullFromStoplight,
  GenerateJenkins, GenerateGitHubAction, GenerateGitLabCI, GenerateCLIRunnerScript,
  SaveGeneratedTest,
  GitHubSavePAT, GitHubDeletePAT, GitHubGetViewer, GitHubListRepositories, GitHubCloneRepository,
  RunRepoAutomation, PickFolder, EnsureProjectForPath,
} from "../../../../wailsjs/go/main/App";
import type { telemetry } from "../../../../wailsjs/go/models";

type SubTab = "github" | "cicd" | "registry" | "telemetry" | "cli";

type GHViewer = {
  login: string;
  name?: string;
  html_url?: string;
};

type GHRepo = {
  full_name: string;
  clone_url: string;
  private: boolean;
  default_branch?: string;
};

export function IntegrationsPanel() {
  const setView = useUIStore((s) => s.setView);
  const [tab, setTab] = useState<SubTab>("cicd");
  const [msg, setMsg] = useState("");

  const tabs: { key: SubTab; label: string }[] = [
    { key: "github", label: "GitHub & Repo Ingest" },
    { key: "cicd", label: "CI/CD Pipelines" },
    { key: "registry", label: "API Registries" },
    { key: "telemetry", label: "Telemetry" },
    { key: "cli", label: "CLI Runner" },
  ];

  return (
    <div className="flex-1 flex flex-col min-w-0 bg-bg">
      <header className="h-[48px] flex items-center gap-3 px-4 border-b border-border shrink-0">
        <button onClick={() => setView("http")} className="text-subtext hover:text-text text-13">&larr; Back</button>
        <h1 className="text-14 font-semibold text-text">Integrations &amp; Extensibility</h1>
      </header>
      <div className="flex gap-1 px-4 py-2 border-b border-border">
        {tabs.map((t) => (
          <button
            key={t.key}
            onClick={() => { setTab(t.key); setMsg(""); }}
            className={`px-3 py-1.5 text-12 rounded-md transition-colors ${
              tab === t.key ? "bg-cyan/10 text-cyan font-semibold" : "text-subtext hover:text-text hover:bg-cardHover"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>
      <div className="flex-1 overflow-y-auto p-4">
        {msg && (
          <div className="mb-3 px-3 py-2 rounded-lg bg-cyan/10 text-cyan text-13 border border-cyan/20">{msg}</div>
        )}
        {tab === "github" && <GitHubIngestTab onMsg={setMsg} />}
        {tab === "cicd" && <CICDTab onMsg={setMsg} />}
        {tab === "registry" && <RegistryTab onMsg={setMsg} />}
        {tab === "telemetry" && <TelemetryTab onMsg={setMsg} />}
        {tab === "cli" && <CLITab onMsg={setMsg} />}
      </div>
    </div>
  );
}

function GitHubIngestTab({ onMsg }: { onMsg: (m: string) => void }) {
  const [account, setAccount] = useState("default");
  const [pat, setPat] = useState("");
  const [viewer, setViewer] = useState<GHViewer | null>(null);
  const [repos, setRepos] = useState<GHRepo[]>([]);
  const [selectedRepo, setSelectedRepo] = useState("");
  const [cloneDest, setCloneDest] = useState("");
  const [repoPath, setRepoPath] = useState("");
  const [outputDir, setOutputDir] = useState(".reqit/scan");
  const [visibility, setVisibility] = useState("all");
  const [busy, setBusy] = useState(false);
  const [lastOutput, setLastOutput] = useState("");

  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    try {
      await fn();
    } catch (e) {
      onMsg(String(e));
    } finally {
      setBusy(false);
    }
  };

  const savePat = () => run(async () => {
    await GitHubSavePAT(account, pat);
    onMsg("PAT saved to OS keychain.");
  });

  const deletePat = () => run(async () => {
    await GitHubDeletePAT(account);
    setViewer(null);
    setRepos([]);
    onMsg("PAT deleted from OS keychain.");
  });

  const verifyAndLoad = () => run(async () => {
    const vRaw = await GitHubGetViewer(account);
    const parsedViewer = JSON.parse(vRaw) as GHViewer;
    setViewer(parsedViewer);

    const raw = await GitHubListRepositories(account, visibility);
    const parsed = JSON.parse(raw) as GHRepo[];
    setRepos(parsed);
    if (parsed.length > 0) {
      setSelectedRepo(parsed[0].clone_url);
    }
    onMsg(`Connected as ${parsedViewer.login}. Loaded ${parsed.length} repositories.`);
  });

  const pickCloneDest = () => run(async () => {
    const dir = await PickFolder("Select clone destination");
    if (dir) {
      setCloneDest(dir);
      onMsg(`Clone destination set: ${dir}`);
    }
  });

  const cloneRepo = () => run(async () => {
    if (!selectedRepo) {
      onMsg("Select a repository first.");
      return;
    }
    if (!cloneDest) {
      onMsg("Select clone destination first.");
      return;
    }
    const clonedPath = await GitHubCloneRepository(account, selectedRepo, cloneDest);
    setRepoPath(clonedPath);
    onMsg(`Cloned repository to ${clonedPath}`);
  });

  const pickRepoPath = () => run(async () => {
    const dir = await PickFolder("Select repository root to ingest");
    if (dir) {
      setRepoPath(dir);
      onMsg(`Repository selected: ${dir}`);
    }
  });

  const runCommand = (command: string) => run(async () => {
    if (!repoPath) {
      onMsg("Select or clone a repository first.");
      return;
    }
    // Repo Ingest still collects a bare filesystem path (a full Project
    // picker UI lands in a later phase) — EnsureProjectForPath finds or
    // creates the Project wrapping it so automation runs through the
    // Project/Source model rather than a raw path.
    const project = await EnsureProjectForPath(repoPath, "");
    const source = project.sources[0];
    if (!source) {
      onMsg("Could not resolve a project source for this repository.");
      return;
    }
    const result = await RunRepoAutomation(command, project.id, source.id, outputDir);
    setLastOutput(result);
    onMsg(result);
  });

  return (
    <div className="space-y-5 max-w-5xl">
      <section className="rounded-xl border border-border bg-surface p-4 space-y-3">
        <h2 className="text-14 font-semibold text-text">1) Connect GitHub Account (PAT)</h2>
        <p className="text-12 text-subtext">
          Create a GitHub personal access token and save it in keychain. Required scopes: <strong>repo</strong> (private repos) and
          <strong> workflow</strong> (if you plan to manage CI workflows).
        </p>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-2">
          <div>
            <label className="text-12 text-subtext block">Account Key</label>
            <input
              value={account}
              onChange={(e) => setAccount(e.target.value)}
              className="w-full px-3 py-1.5 rounded-lg bg-bg border border-border text-13 text-text"
              placeholder="default"
            />
          </div>
          <div className="md:col-span-2">
            <label className="text-12 text-subtext block">Personal Access Token</label>
            <input
              value={pat}
              onChange={(e) => setPat(e.target.value)}
              type="password"
              className="w-full px-3 py-1.5 rounded-lg bg-bg border border-border text-13 text-text"
              placeholder="ghp_..."
            />
          </div>
        </div>
        <div className="flex gap-2 flex-wrap">
          <Button onClick={savePat} disabled={busy || !pat.trim()}>Save PAT</Button>
          <Button onClick={verifyAndLoad} disabled={busy}>Verify & Load Repositories</Button>
          <Button onClick={deletePat} disabled={busy}>Delete PAT</Button>
        </div>
        {viewer && (
          <div className="text-12 text-subtext">Signed in as <span className="text-cyan">{viewer.login}</span>{viewer.name ? ` (${viewer.name})` : ""}</div>
        )}
      </section>

      <section className="rounded-xl border border-border bg-surface p-4 space-y-3">
        <h2 className="text-14 font-semibold text-text">2) Choose Repository</h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-2 items-end">
          <div>
            <label className="text-12 text-subtext block">Visibility</label>
            <select
              value={visibility}
              onChange={(e) => setVisibility(e.target.value)}
              className="w-full px-3 py-1.5 rounded-lg bg-bg border border-border text-13 text-text"
            >
              <option value="all">all</option>
              <option value="public">public</option>
              <option value="private">private</option>
            </select>
          </div>
          <div className="md:col-span-2">
            <label className="text-12 text-subtext block">Repository</label>
            <select
              value={selectedRepo}
              onChange={(e) => setSelectedRepo(e.target.value)}
              className="w-full px-3 py-1.5 rounded-lg bg-bg border border-border text-13 text-text"
            >
              <option value="">Select repository</option>
              {repos.map((r) => (
                <option key={r.clone_url} value={r.clone_url}>
                  {r.full_name} {r.private ? "(private)" : ""}
                </option>
              ))}
            </select>
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-2 items-end">
          <div className="md:col-span-2">
            <label className="text-12 text-subtext block">Clone destination</label>
            <input
              value={cloneDest}
              onChange={(e) => setCloneDest(e.target.value)}
              className="w-full px-3 py-1.5 rounded-lg bg-bg border border-border text-13 text-text"
              placeholder="/Users/you/dev"
            />
          </div>
          <div className="flex gap-2">
            <Button onClick={pickCloneDest} disabled={busy}>Browse</Button>
            <Button onClick={cloneRepo} disabled={busy || !selectedRepo}>Clone</Button>
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-2 items-end">
          <div className="md:col-span-2">
            <label className="text-12 text-subtext block">Repository path (for ingest)</label>
            <input
              value={repoPath}
              onChange={(e) => setRepoPath(e.target.value)}
              className="w-full px-3 py-1.5 rounded-lg bg-bg border border-border text-13 text-text"
              placeholder="/path/to/repo"
            />
          </div>
          <div>
            <Button onClick={pickRepoPath} disabled={busy}>Browse Repo</Button>
          </div>
        </div>
      </section>

      <section className="rounded-xl border border-border bg-surface p-4 space-y-3">
        <h2 className="text-14 font-semibold text-text">3) Run Repo Ingest Commands (Desktop)</h2>
        <div>
          <label className="text-12 text-subtext block">Output directory (relative or absolute)</label>
          <input
            value={outputDir}
            onChange={(e) => setOutputDir(e.target.value)}
            className="w-full max-w-xl px-3 py-1.5 rounded-lg bg-bg border border-border text-13 text-text"
            placeholder=".reqit/scan"
          />
        </div>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
          <Button onClick={() => runCommand("scan")} disabled={busy}>Scan</Button>
          <Button onClick={() => runCommand("generate")} disabled={busy}>Generate</Button>
          <Button onClick={() => runCommand("workflow-install")} disabled={busy}>Install Workflows</Button>
          <Button onClick={() => runCommand("sdk-generate")} disabled={busy}>Generate SDK</Button>
          <Button onClick={() => runCommand("health")} disabled={busy}>Health</Button>
          <Button onClick={() => runCommand("drift")} disabled={busy}>Drift</Button>
          <Button onClick={() => runCommand("report")} disabled={busy}>Report</Button>
          <Button onClick={() => runCommand("full-setup")} disabled={busy}>Full Setup</Button>
        </div>
        {lastOutput && (
          <pre className="text-12 text-subtext bg-bg p-3 rounded-lg whitespace-pre-wrap overflow-x-auto">{lastOutput}</pre>
        )}
        <p className="text-12 text-subtext">
          Full Setup runs: scan, generate, workflow install, sdk generate, health, and report.
        </p>
      </section>
    </div>
  );
}

function CICDTab({ onMsg }: { onMsg: (m: string) => void }) {
  const [runnerFile, setRunnerFile] = useState("runner.js");
  const handleGen = async (fn: (collID: string, runner: string) => Promise<string>) => {
    const { GenerateCLIRunner } = await import("../../../../wailsjs/go/main/App");
    const { useCollectionStore } = await import("@/features/collections/stores/useCollectionStore");
    const colls = useCollectionStore.getState().collections;
    if (colls.length === 0) { onMsg("No collections available. Create one first."); return; }
    const collID = colls[0].id;
    try {
      const content = await fn(collID, runnerFile);
      const path = await SaveGeneratedTest(content, runnerFile);
      if (path) onMsg(`Saved: ${path}`);
    } catch (e) { onMsg(String(e)); }
  };

  return (
    <div className="space-y-4">
      <div>
        <label className="text-13 text-subtext block mb-1">Runner filename</label>
        <input value={runnerFile} onChange={(e) => setRunnerFile(e.target.value)}
          className="w-full max-w-sm px-3 py-1.5 rounded-lg bg-surface border border-border text-13 text-text" />
      </div>
      <div className="grid grid-cols-2 gap-3 max-w-lg">
        <Button onClick={() => handleGen(GenerateGitHubAction)}>Generate GitHub Actions</Button>
        <Button onClick={() => handleGen(GenerateGitLabCI)}>Generate GitLab CI</Button>
        <Button onClick={() => handleGen(GenerateJenkins)}>Generate Jenkins Pipeline</Button>
      </div>
      <p className="text-12 text-subtext mt-1">Generates a pipeline file for the first collection in your workspace. Use the runner file name above as the script path.</p>
    </div>
  );
}

function RegistryTab({ onMsg }: { onMsg: (m: string) => void }) {
  const [apiKey, setApiKey] = useState("");
  const [owner, setOwner] = useState("");
  const [name, setName] = useState("");
  const [version, setVersion] = useState("1.0.0");
  const [projectSlug, setProjectSlug] = useState("");
  const [specJSON, setSpecJSON] = useState("");

  const pushSH = async () => {
    if (!specJSON) { onMsg("Paste a spec JSON first."); return; }
    try {
      const r = await PushToSwaggerHub(specJSON, apiKey, owner, name, version);
      onMsg(`Pushed to SwaggerHub: ${r.url}`);
    } catch (e) { onMsg(String(e)); }
  };
  const pullSH = async () => {
    try {
      const data = await PullFromSwaggerHub(apiKey, owner, name, version);
      setSpecJSON(data);
      onMsg("Pulled from SwaggerHub.");
    } catch (e) { onMsg(String(e)); }
  };
  const pushSL = async () => {
    if (!specJSON) { onMsg("Paste a spec JSON first."); return; }
    try {
      const r = await PushToStoplight(specJSON, apiKey, projectSlug);
      onMsg(`Pushed to Stoplight: ${r.url}`);
    } catch (e) { onMsg(String(e)); }
  };
  const pullSL = async () => {
    try {
      const data = await PullFromStoplight(apiKey, projectSlug);
      setSpecJSON(data);
      onMsg("Pulled from Stoplight.");
    } catch (e) { onMsg(String(e)); }
  };

  return (
    <div className="space-y-4 max-w-lg">
      <div className="grid grid-cols-2 gap-2">
        <div><label className="text-12 text-subtext block">API Key / Token</label>
          <input value={apiKey} onChange={(e) => setApiKey(e.target.value)} className="w-full px-3 py-1.5 rounded-lg bg-surface border border-border text-13 text-text" /></div>
        <div><label className="text-12 text-subtext block">Version</label>
          <input value={version} onChange={(e) => setVersion(e.target.value)} className="w-full px-3 py-1.5 rounded-lg bg-surface border border-border text-13 text-text" /></div>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div><label className="text-12 text-subtext block">SwaggerHub Owner</label>
          <input value={owner} onChange={(e) => setOwner(e.target.value)} className="w-full px-3 py-1.5 rounded-lg bg-surface border border-border text-13 text-text" /></div>
        <div><label className="text-12 text-subtext block">SwaggerHub API Name</label>
          <input value={name} onChange={(e) => setName(e.target.value)} className="w-full px-3 py-1.5 rounded-lg bg-surface border border-border text-13 text-text" /></div>
      </div>
      <div><label className="text-12 text-subtext block">Stoplight Project Slug</label>
        <input value={projectSlug} onChange={(e) => setProjectSlug(e.target.value)} className="w-full px-3 py-1.5 rounded-lg bg-surface border border-border text-13 text-text" /></div>
      <div><label className="text-12 text-subtext block">Spec JSON</label>
        <textarea value={specJSON} onChange={(e) => setSpecJSON(e.target.value)} rows={4} className="w-full px-3 py-1.5 rounded-lg bg-surface border border-border text-13 text-text font-mono" /></div>
      <div className="flex gap-2 flex-wrap">
        <Button onClick={pushSH}>Push to SwaggerHub</Button>
        <Button onClick={pullSH}>Pull from SwaggerHub</Button>
        <Button onClick={pushSL}>Push to Stoplight</Button>
        <Button onClick={pullSL}>Pull from Stoplight</Button>
      </div>
    </div>
  );
}

function TelemetryTab({ onMsg }: { onMsg: (m: string) => void }) {
  const [enabled, setEnabled] = useState(false);
  const [preview, setPreview] = useState("");
  const [events, setEvents] = useState<telemetry.Event[]>([]);

  const refresh = async () => {
    const cfg = await GetTelemetryConfig();
    setEnabled(cfg.enabled);
  };
  useState(() => { refresh(); });

  const toggle = async () => {
    await SetTelemetryEnabled(!enabled);
    setEnabled(!enabled);
    onMsg(`Telemetry ${!enabled ? "enabled" : "disabled"}.`);
  };
  const showPreview = async () => {
    setPreview(await GetTelemetryPreview());
  };
  const showEvents = async () => {
    setEvents(await GetTelemetryEvents(50));
  };

  return (
    <div className="space-y-4 max-w-lg">
      <div className="flex items-center gap-3">
        <span className="text-13 text-text">Telemetry is <strong>{enabled ? "ON" : "OFF"}</strong></span>
        <Button onClick={toggle}>{enabled ? "Disable" : "Enable"}</Button>
      </div>
      {!enabled && (
        <p className="text-12 text-subtext">No data is collected. Enable above to help guide product decisions.</p>
      )}
      <div className="flex gap-2">
        <Button onClick={showPreview}>Preview what is tracked</Button>
        <Button onClick={showEvents}>View recent events</Button>
      </div>
      {preview && (
        <pre className="text-12 text-subtext bg-surface p-3 rounded-lg whitespace-pre-wrap max-h-[300px] overflow-y-auto">{preview}</pre>
      )}
      {events.length > 0 && (
        <div className="space-y-1 max-h-[300px] overflow-y-auto">
          {events.map((e, i) => (
            <div key={i} className="text-12 text-subtext bg-surface px-3 py-1.5 rounded-lg">
              <span className="text-cyan">{e.type}</span> &mdash; {e.name}
              <span className="text-10 text-subtext/50 ml-2">{new Date(e.ts).toLocaleString()}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function CLITab({ onMsg }: { onMsg: (m: string) => void }) {
  const [collName, setCollName] = useState("my-collection");
  const [script, setScript] = useState("");
  const genScript = async () => {
    const s = await GenerateCLIRunnerScript(collName);
    setScript(s);
  };

  return (
    <div className="space-y-4 max-w-lg">
      <div><label className="text-13 text-subtext block mb-1">Collection name</label>
        <input value={collName} onChange={(e) => setCollName(e.target.value)}
          className="w-full px-3 py-1.5 rounded-lg bg-surface border border-border text-13 text-text" /></div>
      <Button onClick={genScript}>Generate CLI Runner Script</Button>
      {script && (
        <>
          <h3 className="text-13 font-semibold text-text mt-3">Shell runner script</h3>
          <pre className="text-12 text-subtext bg-surface p-3 rounded-lg whitespace-pre-wrap overflow-x-auto max-h-[300px]">{script}</pre>
          <p className="text-12 text-subtext">Save this as <code className="text-cyan">{collName}.sh</code> and run with <code className="text-cyan">sh {collName}.sh</code>.</p>
        </>
      )}
    </div>
  );
}
