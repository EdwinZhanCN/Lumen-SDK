import { useState, useEffect } from "react";
import {
  GetStatus,
  GetNodes,
  GetMetrics,
  GetTasks,
  Quit,
  GetConfig,
  SaveConfig,
} from "../bindings/github.com/edwinzhancn/lumen-sdk/cmd/lumen-gateway/gatewayservice";
import { FrontendNodeInfo } from "../bindings/github.com/edwinzhancn/lumen-sdk/cmd/lumen-gateway/models";
import { translations, Language, TranslationKey } from "./i18n";
import {
  Activity,
  Clock,
  Server,
  ShieldAlert,
  Power,
  AlertCircle,
  Database,
  ArrowRight,
  Settings,
  X,
  Check,
  RefreshCw,
  Globe,
  Satellite,
} from "lucide-react";

interface StatusInfo {
  running: boolean;
  uptime: string;
  totalNodes: number;
  activeNodes: number;
  port: number;
  version: string;
  language: string;
}

interface MetricsInfo {
  totalReqs: number;
  successReqs: number;
  failedReqs: number;
  avgLatency: number;
  errorRate: number;
  activeNodes: number;
}

function App() {
  // UI states
  const [lang, setLang] = useState<Language>("zh");
  const [isLangInitialized, setIsLangInitialized] = useState<boolean>(false);
  const [isSettingsOpen, setIsSettingsOpen] = useState<boolean>(false);
  const [activeTab, setActiveTab] = useState<"nodes" | "tasks">("nodes");
  const [isSaving, setIsSaving] = useState<boolean>(false);
  const [saveSuccess, setSaveSuccess] = useState<boolean>(false);

  // App core states
  const [status, setStatus] = useState<StatusInfo>({
    running: false,
    uptime: "0s",
    totalNodes: 0,
    activeNodes: 0,
    port: 5866,
    version: "1.0.0",
    language: "zh",
  });
  const [nodes, setNodes] = useState<FrontendNodeInfo[]>([]);
  const [metrics, setMetrics] = useState<MetricsInfo>({
    totalReqs: 0,
    successReqs: 0,
    failedReqs: 0,
    avgLatency: 0,
    errorRate: 0,
    activeNodes: 0,
  });
  const [tasks, setTasks] = useState<Record<string, any>>({});

  // Settings form states
  const [formPort, setFormPort] = useState<number>(5866);
  const [formScanInterval, setFormScanInterval] = useState<string>("30s");
  const [formHubUrl, setFormHubUrl] = useState<string>("");
  const [formLogLevel, setFormLogLevel] = useState<string>("info");

  // Translation helper
  const t = (key: TranslationKey): string => {
    return translations[lang][key] || translations["en"][key] || String(key);
  };

  // Fetch metrics and node states
  const updateState = async () => {
    try {
      const currentStatus = await GetStatus();
      if (currentStatus) {
        setStatus(currentStatus as StatusInfo);
        // Sync language from backend settings on initial load only
        if (currentStatus.language && !isLangInitialized) {
          setLang(currentStatus.language as Language);
          setIsLangInitialized(true);
        }
      }

      const currentMetrics = await GetMetrics();
      if (currentMetrics) setMetrics(currentMetrics as MetricsInfo);

      const currentNodes = await GetNodes();
      if (currentNodes) setNodes(currentNodes);

      const currentTasks = await GetTasks();
      if (currentTasks) setTasks(currentTasks);
    } catch (err) {
      console.error("Failed to fetch state:", err);
    }
  };

  // Fetch full configuration for settings form
  const loadConfigData = async () => {
    try {
      const config = await GetConfig();
      if (config) {
        setFormPort(config.restPort || 5866);
        setFormScanInterval(config.scanInterval || "30s");
        setFormHubUrl(config.hubUrl || "");
        setFormLogLevel(config.logLevel || "info");
      }
    } catch (err) {
      console.error("Failed to load configuration:", err);
    }
  };

  useEffect(() => {
    updateState();
    loadConfigData();
    const interval = setInterval(updateState, 2000);
    return () => clearInterval(interval);
  }, []);

  // When settings panel opens, reload configuration to match backend precisely
  useEffect(() => {
    if (isSettingsOpen) {
      loadConfigData();
    }
  }, [isSettingsOpen]);

  const handleQuit = () => {
    Quit();
  };

  const handleSaveSettings = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSaving(true);
    setSaveSuccess(false);

    try {
      await SaveConfig({
        port: formPort,
        scanInterval: formScanInterval,
        hubUrl: formHubUrl,
        logLevel: formLogLevel,
        language: lang,
      });

      setSaveSuccess(true);
      // Fast state refresh to sync with restarted daemon
      await updateState();

      // Auto close settings after showing success message
      setTimeout(() => {
        setSaveSuccess(false);
        setIsSettingsOpen(false);
      }, 1500);
    } catch (err) {
      console.error("Failed to save settings:", err);
      alert("Error saving settings: " + err);
    } finally {
      setIsSaving(false);
    }
  };

  const formatTaskName = (name: string) => {
    return name
      .replace(/_/g, " ")
      .replace(/\b\w/g, (char) => char.toUpperCase());
  };

  return (
    <div className="w-[360px] h-[500px] bg-slate-950 text-slate-100 flex flex-col font-sans select-none overflow-hidden border border-slate-800 rounded-lg shadow-2xl relative">
      {/* Settings Panel Overlay */}
      {isSettingsOpen && (
        <div className="absolute inset-0 bg-slate-950 z-50 flex flex-col animate-in fade-in slide-in-from-bottom-5 duration-200">
          <header className="px-4 py-3 bg-slate-900 border-b border-slate-800 flex items-center justify-between">
            <h2 className="text-sm font-semibold flex items-center gap-1.5">
              <Settings className="w-4 h-4 text-indigo-400" />
              {t("settingsTitle")}
            </h2>
            <button
              onClick={() => setIsSettingsOpen(false)}
              className="p-1 text-slate-400 hover:text-slate-200 hover:bg-slate-800 rounded transition"
            >
              <X className="w-4 h-4" />
            </button>
          </header>

          <form
            onSubmit={handleSaveSettings}
            className="flex-1 overflow-y-auto p-4 space-y-4 text-xs"
          >
            {/* Port */}
            <div className="space-y-1.5">
              <label className="text-slate-400 font-medium">
                {t("portLabel")}
              </label>
              <input
                type="number"
                value={formPort}
                onChange={(e) => setFormPort(parseInt(e.target.value))}
                className="w-full bg-slate-900 border border-slate-800 rounded px-2.5 py-1.5 text-slate-200 focus:outline-none focus:border-indigo-500 transition"
                required
              />
            </div>

            {/* Scan Interval */}
            <div className="space-y-1.5">
              <label className="text-slate-400 font-medium">
                {t("scanIntervalLabel")}
              </label>
              <select
                value={formScanInterval}
                onChange={(e) => setFormScanInterval(e.target.value)}
                className="w-full bg-slate-900 border border-slate-800 rounded px-2.5 py-1.5 text-slate-200 focus:outline-none focus:border-indigo-500 transition"
              >
                <option value="15s">15s</option>
                <option value="30s">30s</option>
                <option value="60s">60s</option>
                <option value="5m">5m</option>
              </select>
            </div>

            {/* Hub URL */}
            <div className="space-y-1.5">
              <label className="text-slate-400 font-medium">
                {t("hubUrlLabel")}
              </label>
              <input
                type="text"
                value={formHubUrl}
                onChange={(e) => setFormHubUrl(e.target.value)}
                placeholder="http://example-hub:5866"
                className="w-full bg-slate-900 border border-slate-800 rounded px-2.5 py-1.5 text-slate-200 focus:outline-none focus:border-indigo-500 transition placeholder-slate-600"
              />
            </div>

            {/* Log Level */}
            <div className="space-y-1.5">
              <label className="text-slate-400 font-medium">
                {t("logLevelLabel")}
              </label>
              <select
                value={formLogLevel}
                onChange={(e) => setFormLogLevel(e.target.value)}
                className="w-full bg-slate-900 border border-slate-800 rounded px-2.5 py-1.5 text-slate-200 focus:outline-none focus:border-indigo-500 transition"
              >
                <option value="debug">Debug</option>
                <option value="info">Info</option>
                <option value="warn">Warn</option>
                <option value="error">Error</option>
              </select>
            </div>

            {/* Language */}
            <div className="space-y-1.5">
              <label className="text-slate-400 font-medium flex items-center gap-1">
                <Globe className="w-3.5 h-3.5 text-slate-500" />
                {t("languageLabel")}
              </label>
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => setLang("zh")}
                  className={`flex-1 py-1.5 border rounded font-semibold transition ${lang === "zh" ? "bg-indigo-600 border-indigo-500 text-white" : "border-slate-800 text-slate-400 hover:bg-slate-900"}`}
                >
                  中文 (简体)
                </button>
                <button
                  type="button"
                  onClick={() => setLang("en")}
                  className={`flex-1 py-1.5 border rounded font-semibold transition ${lang === "en" ? "bg-indigo-600 border-indigo-500 text-white" : "border-slate-800 text-slate-400 hover:bg-slate-900"}`}
                >
                  English
                </button>
              </div>
            </div>

            {/* Settings Actions */}
            <div className="pt-2 flex flex-col gap-2">
              <button
                type="submit"
                disabled={isSaving}
                className="w-full bg-indigo-600 text-white font-semibold py-2 rounded hover:bg-indigo-500 active:bg-indigo-700 transition flex items-center justify-center gap-1.5 disabled:opacity-50"
              >
                {isSaving ? (
                  <>
                    <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                    {t("saving")}
                  </>
                ) : (
                  <>
                    <Check className="w-3.5 h-3.5" />
                    {t("saveBtn")}
                  </>
                )}
              </button>

              <button
                type="button"
                onClick={() => setIsSettingsOpen(false)}
                className="w-full border border-slate-800 text-slate-400 font-semibold py-2 rounded hover:bg-slate-900 transition"
              >
                {t("closeSettings")}
              </button>
            </div>
          </form>

          {/* Success Notification Alert */}
          {saveSuccess && (
            <div className="absolute inset-x-4 bottom-4 bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-xs py-2.5 px-3 rounded-lg flex items-center gap-2 animate-in fade-in slide-in-from-bottom-2 duration-150">
              <Check className="w-4 h-4 text-emerald-400 shrink-0" />
              <span>{t("successSave")}</span>
            </div>
          )}
        </div>
      )}

      {/* Main Header */}
      <header className="px-4 py-3 bg-slate-900/80 backdrop-blur border-b border-slate-800 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <div className="p-1.5 bg-indigo-600 rounded-md">
            <Satellite className="w-4 h-4 text-white" />
          </div>
          <div>
            <h1 className="text-sm font-semibold tracking-wide">
              {t("title")}
            </h1>
            <div className="flex items-center gap-1.5">
              <span
                className={`w-1.5 h-1.5 rounded-full ${status.running ? "bg-emerald-500 animate-pulse" : "bg-rose-500"}`}
              ></span>
              <span className="text-[10px] text-slate-400 font-medium">
                {status.running
                  ? `${t("active")} • Port ${status.port}`
                  : t("stopped")}
              </span>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-1">
          <button
            onClick={() => setIsSettingsOpen(true)}
            title={t("settingsTitle")}
            className="p-1.5 text-slate-400 hover:text-indigo-400 hover:bg-slate-800 rounded-md transition-all duration-200"
          >
            <Settings className="w-4 h-4" />
          </button>
          <button
            onClick={handleQuit}
            title={t("quitBtn")}
            className="p-1.5 text-slate-400 hover:text-rose-400 hover:bg-slate-800 rounded-md transition-all duration-200"
          >
            <Power className="w-4 h-4" />
          </button>
        </div>
      </header>

      {/* Metrics Dashboard */}
      <section className="p-4 grid grid-cols-2 gap-3 bg-slate-950">
        <div className="bg-slate-900/40 border border-slate-800/60 rounded-xl p-3 flex flex-col justify-between">
          <div className="flex items-center justify-between text-slate-400 mb-1">
            <span className="text-[11px] font-medium">{t("requests")}</span>
            <Activity className="w-3.5 h-3.5 text-indigo-400" />
          </div>
          <div>
            <span className="text-lg font-bold tracking-tight">
              {metrics.totalReqs.toLocaleString()}
            </span>
            <span className="text-[10px] text-slate-400 ml-1">
              <span className="text-emerald-400">{metrics.successReqs}</span>
              <span className="mx-0.5">/</span>
              <span className="text-amber-400">{metrics.failedReqs}</span>
            </span>
          </div>
        </div>

        <div className="bg-slate-900/40 border border-slate-800/60 rounded-xl p-3 flex flex-col justify-between">
          <div className="flex items-center justify-between text-slate-400 mb-1">
            <span className="text-[11px] font-medium">{t("avgLatency")}</span>
            <Clock className="w-3.5 h-3.5 text-emerald-400" />
          </div>
          <div>
            <span className="text-lg font-bold tracking-tight">
              {metrics.avgLatency > 0 ? metrics.avgLatency.toFixed(1) : "-"}
            </span>
            <span className="text-[10px] text-slate-400 ml-1">ms</span>
          </div>
        </div>

        <div className="bg-slate-900/40 border border-slate-800/60 rounded-xl p-3 flex flex-col justify-between">
          <div className="flex items-center justify-between text-slate-400 mb-1">
            <span className="text-[11px] font-medium">{t("activeNodes")}</span>
            <Server className="w-3.5 h-3.5 text-blue-400" />
          </div>
          <div>
            <span className="text-lg font-bold tracking-tight">
              {status.activeNodes}
            </span>
            <span className="text-[10px] text-slate-400 ml-1">
              / {status.totalNodes}
            </span>
          </div>
        </div>

        <div className="bg-slate-900/40 border border-slate-800/60 rounded-xl p-3 flex flex-col justify-between">
          <div className="flex items-center justify-between text-slate-400 mb-1">
            <span className="text-[11px] font-medium">{t("errorRate")}</span>
            <ShieldAlert className="w-3.5 h-3.5 text-amber-400" />
          </div>
          <div>
            <span className="text-lg font-bold tracking-tight">
              {metrics.errorRate.toFixed(1)}
            </span>
            <span className="text-[10px] text-slate-400 ml-1">%</span>
          </div>
        </div>
      </section>

      {/* Tabs */}
      <div className="px-4 border-b border-slate-800 flex gap-4 text-xs font-semibold bg-slate-950">
        <button
          onClick={() => setActiveTab("nodes")}
          className={`pb-2 border-b-2 transition-all ${activeTab === "nodes" ? "border-indigo-500 text-indigo-400" : "border-transparent text-slate-400 hover:text-slate-200"}`}
        >
          {t("nodesTab")} ({nodes.length})
        </button>
        <button
          onClick={() => setActiveTab("tasks")}
          className={`pb-2 border-b-2 transition-all ${activeTab === "tasks" ? "border-indigo-500 text-indigo-400" : "border-transparent text-slate-400 hover:text-slate-200"}`}
        >
          {t("tasksTab")}
        </button>
      </div>

      {/* Content Area */}
      <div className="flex-1 overflow-y-auto px-4 py-3 bg-slate-950/60">
        {activeTab === "nodes" ? (
          nodes.length === 0 ? (
            <div className="h-full flex flex-col items-center justify-center text-slate-500 text-xs py-8">
              <AlertCircle className="w-8 h-8 mb-2 text-slate-600 animate-pulse" />
              <p>{t("noNodes")}</p>
              <p className="text-[10px] text-slate-600 mt-1">
                {t("noNodesSub")}
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              {nodes.map((node) => (
                <div
                  key={node.id}
                  className="bg-slate-900 border border-slate-800/80 rounded-xl p-3 space-y-2"
                >
                  {/* Node Header */}
                  <div className="flex justify-between items-start">
                    <div>
                      <h3
                        className="text-xs font-bold text-slate-200 tracking-wide truncate max-w-[200px]"
                        title={node.name}
                      >
                        {node.name}
                      </h3>
                      <p className="text-[10px] text-slate-400">
                        {node.address}
                      </p>
                    </div>
                    <span
                      className={`text-[9px] px-2 py-0.5 rounded-full font-bold uppercase tracking-wider ${
                        node.status === "active"
                          ? "bg-emerald-500/10 text-emerald-400 border border-emerald-500/20"
                          : node.status === "starting"
                            ? "bg-amber-500/10 text-amber-400 border border-amber-500/20 animate-pulse"
                            : "bg-rose-500/10 text-rose-400 border border-rose-500/20"
                      }`}
                    >
                      {node.status === "active"
                        ? t("statusActive")
                        : node.status === "starting"
                          ? t("statusStarting")
                          : t("statusOffline")}
                    </span>
                  </div>

                  {/* Task Tags */}
                  {node.tasks && node.tasks.length > 0 && (
                    <div className="flex flex-wrap gap-1 pt-1.5">
                      {node.tasks.slice(0, 3).map((task) => (
                        <span
                          key={task}
                          className="text-[9px] bg-slate-800 text-slate-300 px-1.5 py-0.5 rounded font-medium border border-slate-700/50"
                        >
                          {formatTaskName(task)}
                        </span>
                      ))}
                      {node.tasks.length > 3 && (
                        <span className="text-[9px] bg-indigo-950 text-indigo-300 px-1.5 py-0.5 rounded font-medium border border-indigo-900/50">
                          +{node.tasks.length - 3} {t("more")}
                        </span>
                      )}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )
        ) : /* Tasks tab */
        Object.keys(tasks).length === 0 ? (
          <div className="h-full flex flex-col items-center justify-center text-slate-500 text-xs py-8">
            <Database className="w-8 h-8 mb-2 text-slate-600" />
            <p>{t("noTasks")}</p>
            <p className="text-[10px] text-slate-600 mt-1">{t("noTasksSub")}</p>
          </div>
        ) : (
          <div className="space-y-4">
            {Object.keys(tasks).map((serviceName) => (
              <div key={serviceName} className="space-y-2">
                <h3 className="text-[11px] font-bold text-indigo-400 uppercase tracking-wider flex items-center gap-1.5">
                  <span className="w-1.5 h-1.5 rounded-full bg-indigo-400"></span>
                  {serviceName}
                </h3>
                <div className="space-y-1.5">
                  {tasks[serviceName].map((task: any, index: number) => (
                    <div
                      key={index}
                      className="bg-slate-900/50 border border-slate-850 p-2.5 rounded-lg flex items-center justify-between text-xs hover:bg-slate-900 transition duration-150"
                    >
                      <div>
                        <div className="font-semibold text-slate-200">
                          {formatTaskName(task.name)}
                        </div>
                        <div className="text-[9px] text-slate-500 mt-0.5 flex items-center gap-1">
                          <span>{t("supportedBy")}:</span>
                          <span className="text-slate-400 font-medium">
                            {task.node_name}
                          </span>
                        </div>
                      </div>
                      <ArrowRight className="w-3.5 h-3.5 text-slate-500" />
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Footer */}
      <footer className="px-4 py-2 border-t border-slate-950 bg-slate-900/30 flex justify-between items-center text-[10px] text-slate-500 font-medium">
        <span>
          {t("uptime")}: {status.uptime}
        </span>
        <span>Lumen Gateway {status.version}</span>
      </footer>
    </div>
  );
}

export default App;
