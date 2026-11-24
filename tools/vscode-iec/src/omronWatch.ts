import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { createInterface } from "node:readline";
import * as vscode from "vscode";
import { cliCommand } from "./cli";
import { endpointError } from "./omronBuild";

type WatchBinding = { name: string; address: string; type: string };
type WatchModel = { bindings: WatchBinding[]; error?: string };
type WatchSample = {
  cpu: { connected: boolean; mode: number; lastError?: string };
  values: Record<string, number | boolean>;
  quality: Record<string, string>;
};

export class OmronWatch implements vscode.Disposable {
  private config?: WatchModel;
  private child?: ChildProcessWithoutNullStreams;
  private request?: vscode.CancellationTokenSource;
  private lastEndpoint?: string;
  private silence?: NodeJS.Timeout;

  constructor(private readonly webview: vscode.Webview) {}

  bind(model: unknown): void {
    this.config = (model as { omron?: WatchModel } | undefined)?.omron;
  }

  async toggle(): Promise<void> {
    if (this.child) {
      this.stop();
      this.post(false);
      return;
    }
    if (this.request || !this.config?.bindings.length || this.config.error) return;
    if (!vscode.workspace.isTrusted) {
      this.post(false, undefined, "Omron monitoring requires a trusted workspace.");
      return;
    }
    const request = new vscode.CancellationTokenSource();
    this.request = request;
    let target: string | undefined;
    try {
      target = await vscode.window.showInputBox({
        title: "Omron CJ1 Watch",
        prompt: "Read-only FINS/TCP endpoint",
        placeHolder: "192.0.2.10:9600",
        value: this.lastEndpoint,
        validateInput: endpointError,
      }, request.token);
      if (!target || request.token.isCancellationRequested) return;
      this.start(target.trim());
    } finally {
      if (this.request === request) this.request = undefined;
      request.dispose();
    }
  }

  private start(endpoint: string): void {
    const bindings = this.config?.bindings;
    if (!bindings?.length || this.config?.error || this.child) return;
    const invalid = endpointError(endpoint);
    if (invalid) {
      this.post(false, undefined, invalid);
      return;
    }
    this.lastEndpoint = endpoint;
    const child = spawn(cliCommand(), [
      "omron", "watch", "-transport", "tcp", "-endpoint", endpoint, "-tags", "-", "-poll", "500ms",
    ], { windowsHide: true });
    this.child = child;
    this.post(true);
    const lines = createInterface({ input: child.stdout });
    let stderr = "";
    child.stderr.setEncoding("utf8");
    child.stderr.on("data", (chunk: string) => { stderr = (stderr + chunk).slice(-2048); });
    child.stdin.on("error", () => {});
    child.stdin.end(JSON.stringify(bindings));
    lines.on("line", (line) => {
      if (this.child !== child) return;
      try {
        const sample = JSON.parse(line) as WatchSample;
        if (!sample.cpu || !sample.values || !sample.quality) throw new Error("Invalid sample");
        this.post(true, sample);
        clearTimeout(this.silence);
        this.silence = setTimeout(() => this.post(true, undefined, "Waiting for a fresh sample…"), 4500);
      } catch {
        this.stop();
        this.post(false, undefined, "Invalid data from naut omron watch.");
      }
    });
    const failed = (reason: string) => {
      if (this.child !== child) return;
      this.stop();
      this.post(false, undefined, reason);
    };
    child.once("error", (error) => failed(error.message));
    child.once("close", (code) => failed(stderr.trim() || `Omron watch exited (${code ?? "signal"}).`));
  }

  private post(active: boolean, sample?: WatchSample, error?: string): void {
    void this.webview.postMessage({ type: "omronWatch", active, endpoint: this.lastEndpoint, sample, error });
  }

  private stop(): void {
    clearTimeout(this.silence);
    const child = this.child;
    this.child = undefined;
    child?.kill();
  }

  reset(): void {
    this.dispose();
    this.post(false);
  }

  dispose(): void {
    this.config = undefined;
    this.request?.cancel();
    this.request = undefined;
    this.stop();
  }
}
