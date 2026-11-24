import { execFile } from "node:child_process";
import * as fs from "node:fs/promises";
import { isIP } from "node:net";
import * as path from "node:path";
import { promisify } from "node:util";
import * as vscode from "vscode";
import { cliCommand, cliExecOptions } from "./cli";

const execCLI = promisify(execFile);

async function saveProjectInputs(root: vscode.Uri, names: string[]): Promise<void> {
  for (const doc of vscode.workspace.textDocuments) {
    if (!doc.isDirty || doc.uri.scheme !== root.scheme || doc.uri.authority !== root.authority) continue;
    if (names.some(name => path.relative(path.join(root.fsPath, name), doc.uri.fsPath) === "") && !await doc.save()) {
      throw new Error(`Could not save ${doc.fileName}`);
    }
  }
}

async function readProjectJSON(root: vscode.Uri, name: string) {
  const bytes = await vscode.workspace.fs.readFile(vscode.Uri.joinPath(root, name));
  return parseProjectJSON(name, bytes);
}

function parseProjectJSON(name: string, bytes: Uint8Array) {
  try {
    return JSON.parse(Buffer.from(bytes).toString("utf8"));
  } catch (err) {
    throw new Error(`${name}: ${err instanceof Error ? err.message : String(err)}`);
  }
}

function pickCJ1Folder() {
  const roots = vscode.workspace.workspaceFolders ?? [];
  return roots.length === 1 ? roots[0] : vscode.window.showWorkspaceFolderPick({ placeHolder: "Choose the folder containing cj1.json" });
}

const programOnly = "Download and leave in PROGRAM";
const testRun = "Test RUN and restore original";
const modelNames: Readonly<Record<string, string>> = {
  "CJ1M CPU21": "CJ1M_CPU21",
  "CJ1H-H CPU65": "CJ1H_CPU65H",
};

export function endpointError(input: string): string | undefined {
  const match = /^(\[[^\]]+\]|[^:]+):(\d+)$/.exec(input.trim());
  if (match && isIP(match[1].replace(/^\[|\]$/g, "")) === (match[1].startsWith("[") ? 6 : 4) &&
      Number(match[2]) >= 1 && Number(match[2]) <= 65535) return;
  return "Enter an IPv4 address or bracketed IPv6 address and a port (e.g. 192.0.2.10:9600).";
}

interface CJ1Status {
  cpu: { model: string; version: string; raw: string };
  status: { mode: number; status: number; fatal: number; nonFatal: number; raw: string };
}

interface CJ1DownloadReport {
  session: {
    state: string;
    finished: boolean;
    observedExecutionMode?: number;
    candidate: { readback?: { exact: boolean } };
  };
  taskFlags?: string;
  originalVerified?: boolean;
}

interface DownloadPrompts {
  showQuickPick(items: readonly string[], options: vscode.QuickPickOptions): Thenable<string | undefined>;
  showInputBox(options: vscode.InputBoxOptions): Thenable<string | undefined>;
  showOpenDialog(options: vscode.OpenDialogOptions): Thenable<vscode.Uri[] | undefined>;
  showWarningMessage(message: string, options: vscode.MessageOptions, button: string): Thenable<string | undefined>;
}

async function pickUM(prompts: DownloadPrompts, title: string) {
  const files = await prompts.showOpenDialog({ title, filters: { "User memory image": ["um", "bin", "n1"] } });
  return files?.[0];
}

export async function downloadCJ1Project(root: vscode.WorkspaceFolder, output: vscode.OutputChannel, prompts: DownloadPrompts = vscode.window): Promise<void> {
  if (!vscode.workspace.isTrusted) throw new Error("Omron downloads require a trusted workspace.");
  const manifestURI = vscode.Uri.joinPath(root.uri, "cj1.json");
  await saveProjectInputs(root.uri, ["cj1.json"]);
  const manifestBytes = await vscode.workspace.fs.readFile(manifestURI);
  const manifest = parseProjectJSON("cj1.json", manifestBytes);
  if (manifest.sourceFormat !== undefined && manifest.sourceFormat !== "") {
    throw new Error("Omron download requires a CXT ladder project.");
  }
  if (!Object.prototype.hasOwnProperty.call(modelNames, manifest.model)) throw new Error("Choose a supported CJ1 model in cj1.json.");
  await saveProjectInputs(root.uri, ["program.cxt", "settings.json"]);
  const [source, settings] = await Promise.all(["program.cxt", "settings.json"].map(name =>
    vscode.workspace.fs.readFile(vscode.Uri.joinPath(root.uri, name))));

  const mode = await prompts.showQuickPick([programOnly, testRun], { title: "Omron CJ1 Download" });
  if (!mode) return;
  const endpoint = (await prompts.showInputBox({
    title: "Omron CJ1 Target", prompt: "Direct FINS/TCP IP address and port", validateInput: endpointError,
  }))?.trim();
  if (!endpoint || endpointError(endpoint)) return;

  const compiled = "Compare compiled bytes";
  const saved = "Use a saved new4 resident UM";
  const referenceMode = await prompts.showQuickPick([compiled, saved], { title: "Omron CJ1 Readback Reference" });
  if (!referenceMode) return;
  const reference = referenceMode === saved ? await pickUM(prompts, "Select the verified new4 resident UM") : undefined;
  if (referenceMode === saved && !reference) return;

  let original: vscode.Uri | undefined;
  let taskCount = "0";
  if (mode === testRun) {
    original = await pickUM(prompts, "Select the exact original UM to restore");
    if (!original) return;
    const count = await prompts.showInputBox({
      title: "Omron CJ1 Task Check", prompt: "Number of active task flags to verify from TK0 (0 skips this check)",
      value: "0", validateInput: value => /^(?:[0-9]|[12][0-9]|3[0-2])$/.test(value) ? undefined : "Enter a number from 0 to 32.",
    });
    if (count === undefined) return;
    taskCount = count;
  }
  const [referenceBytes, originalBytes] = await Promise.all([
    reference && vscode.workspace.fs.readFile(reference), original && vscode.workspace.fs.readFile(original),
  ]);

  const cli = cliCommand();
  const { stdout: targetJSON } = await execCLI(cli, ["omron", "status", "-transport", "tcp", "-endpoint", endpoint], cliExecOptions());
  const target: CJ1Status = JSON.parse(targetJSON);
  if (target.cpu?.model !== modelNames[manifest.model] || !target.cpu.raw || !target.status?.raw) {
    throw new Error(`CPU at ${endpoint} reports ${target.cpu?.model ?? "unknown"}; project targets ${manifest.model}.`);
  }
  const state = target.status;
  if (state.mode !== 0 || (state.status & 1) !== 0 || state.fatal !== 0 || state.nonFatal !== 0) {
    throw new Error(`PLC at ${endpoint} must be in clean PROGRAM before downloading.`);
  }

  const action = mode === testRun ? "Run Test and Restore" : "Download to PLC";
  const version = target.cpu.version?.split("\0").filter(Boolean).join(" / ");
  const detail = [
    `Project: ${root.name}`,
    `Controller: ${target.cpu.model}${version ? " (" + version + ")" : ""} at ${endpoint}`,
    mode === testRun
      ? `This replaces the PLC program, starts RUN and restores ${original!.fsPath} in PROGRAM. Confirm the bench is isolated before starting.`
      : "This replaces the PLC program and leaves the controller in PROGRAM.",
  ].join("\n");
  if (await prompts.showWarningMessage(`${action}?`, { modal: true, detail }, action) !== action) return;

  await vscode.window.withProgress({ location: vscode.ProgressLocation.Notification, title: `Omron CJ1: ${mode}`, cancellable: false }, async () => {
    const dir = path.join(root.uri.fsPath, "build");
    await fs.mkdir(dir, { recursive: true });
    const job = await fs.mkdtemp(path.join(dir, "download-"));
    output.show(true);
    output.appendLine(`${mode} at ${endpoint}\nJournal: ${path.join(job, "journal")}`);
    const targetFile = path.join(job, "target.json");
    const args = ["omron", "download", "-project", "-endpoint", endpoint, "-target", targetFile, "-journal", path.join(job, "journal"), "-execute"];
    // Pass only the saved bytes that were selected before the operator confirmed.
    await Promise.all([
      fs.writeFile(path.join(job, "cj1.json"), manifestBytes, { flag: "wx" }),
      fs.writeFile(path.join(job, "program.cxt"), source, { flag: "wx" }),
      fs.writeFile(path.join(job, "settings.json"), settings, { flag: "wx" }),
      fs.writeFile(targetFile, targetJSON, { flag: "wx" }),
    ]);
    if (referenceBytes) {
      const file = path.join(job, "resident-reference.um");
      await fs.writeFile(file, referenceBytes, { flag: "wx" });
      args.push("-resident-reference", file);
    }
    if (originalBytes) {
      const file = path.join(job, "expected-original.um");
      await fs.writeFile(file, originalBytes, { flag: "wx" });
      args.push("-run", "-check-task-flags", taskCount, "-return-original", "-expected-original", file);
    }
    args.push(job);

    let stdout: string;
    try {
      // The CLI bounds network exchanges and owns trial recovery. A VS Code
      // process timeout could interrupt a restore in progress.
      ({ stdout } = await execCLI(cli, args, cliExecOptions({ timeoutMs: 0 })));
    } catch (error) {
      const failure = error as Error & { stdout?: string; stderr?: string };
      if (failure.stdout) {
        output.appendLine(failure.stdout.trim());
        await fs.writeFile(path.join(job, "result.json"), failure.stdout, { flag: "wx" }).catch(error => {
          output.appendLine(`Could not save report: ${String(error)}`);
        });
      }
      throw new Error(failure.stderr?.trim() || failure.message);
    }
    output.appendLine(stdout.trim());
    await fs.writeFile(path.join(job, "result.json"), stdout, { flag: "wx" });
    const report: CJ1DownloadReport = JSON.parse(stdout);
    const bench = mode === testRun;
    if (!report.session?.finished || report.session.candidate?.readback?.exact !== true ||
        (bench
          ? !report.originalVerified || report.session.observedExecutionMode !== 4 ||
            (Number(taskCount) > 0 && report.taskFlags !== "01".repeat(Number(taskCount)))
          : report.session.state !== "candidate-matched")) {
      throw new Error("Omron download did not report the requested verified outcome. See the journal.");
    }
    const tasks = Number(taskCount) > 0 ? `, verified TK0–TK${Number(taskCount) - 1}` : "";
    const message = bench ? `RUN verified${tasks}. Original restored in PROGRAM.` : "Download verified. PLC is in PROGRAM.";
    output.appendLine(message);
    void vscode.window.showInformationMessage(message);
  });
}

export function registerCJ1Build(): vscode.Disposable {
  const output = vscode.window.createOutputChannel("Nautilus CJ1");
  const command = vscode.commands.registerCommand("nautilus.omron.buildProject", async () => {
    const root = await pickCJ1Folder();
    if (!root) return;
    output.show(true);
    try {
      await saveProjectInputs(root.uri, ["cj1.json"]);
      const manifest = await readProjectJSON(root.uri, "cj1.json");
      if (manifest.sourceFormat === "st-il") {
        await saveProjectInputs(root.uri, ["program.json", "settings.json"]);
        const program = await readProjectJSON(root.uri, "program.json");
        const names: string[] = [];
        for (const def of program.definitions ?? []) {
          if (typeof def?.name !== "string" || !/^[A-Za-z][A-Za-z0-9_]{0,63}$/.test(def.name)) {
            throw new Error("program.json contains an invalid ST definition name");
          }
          names.push(def.name + ".st");
        }
        await saveProjectInputs(root.uri, names);
      } else if (manifest.sourceFormat === undefined) {
        await saveProjectInputs(root.uri, ["program.cxt", "settings.json"]);
      }
      const {stdout} = await promisify(execFile)(cliCommand(), ["omron", "build-project", root.uri.fsPath], cliExecOptions({timeoutMs: 60_000}));
      output.appendLine(stdout.trim());
      const report = JSON.parse(stdout);
      void vscode.window.showInformationMessage(`Built ${report.model}: ${report.output}`);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      output.appendLine(message);
      void vscode.window.showErrorMessage(message);
    }
  });
  let downloading = false;
  const download = vscode.commands.registerCommand("nautilus.omron.downloadProject", async () => {
    if (downloading) {
      void vscode.window.showWarningMessage("An Omron download is already in progress.");
      return;
    }
    downloading = true;
    try {
      const root = await pickCJ1Folder();
      if (root) await downloadCJ1Project(root, output);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      output.show(true);
      output.appendLine(message);
      void vscode.window.showErrorMessage(message);
    } finally {
      downloading = false;
    }
  });
  const status = vscode.commands.registerCommand("nautilus.omron.readStatus", async () => {
    const endpoint = (await vscode.window.showInputBox({prompt: "CJ1 FINS/TCP endpoint (host:port)"}))?.trim();
    if (!endpoint) return;
    output.show(true);
    try {
      const {stdout} = await promisify(execFile)(cliCommand(), ["omron", "status", "-diagnostic", "-transport", "tcp", "-endpoint", endpoint], cliExecOptions());
      const report = JSON.parse(stdout);
      const mode = ({0: "PROGRAM", 2: "MONITOR", 4: "RUN"} as Record<number, string>)[report.status.mode];
      const hex = (value: number) => value.toString(16).toUpperCase().padStart(4, "0");
      output.appendLine(`${endpoint}  ${mode}  Executing: ${report.executing}  Fatal: ${hex(report.status.fatal)}  Nonfatal: ${hex(report.status.nonFatal)}  CPU error: ${hex(report.errorCode)}`);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      output.appendLine(message);
      void vscode.window.showErrorMessage(message);
    }
  });
  return vscode.Disposable.from(command, download, status, output);
}
