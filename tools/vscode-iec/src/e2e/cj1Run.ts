import { execFileSync } from "node:child_process";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { runTests } from "@vscode/test-electron";

async function main(): Promise<void> {
  const extension = path.resolve(__dirname, "../..");
  const repo = path.resolve(extension, "../..");
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "nautilus-cj1-"));
  try {
    const cli = process.env.NAUTILUS_E2E_BIN ?? path.join(tmp, process.platform === "win32" ? "naut.exe" : "naut");
    if (!process.env.NAUTILUS_E2E_BIN) {
      execFileSync("go", ["build", "-o", cli, "./cmd/naut"], { cwd: repo });
    }
    const fixtures = path.join(repo, "omron/compiler/testdata");
    for (const [model, family] of [["CJ1M CPU21", ""], ["CJ1M CPU21", "cj1m"], ["CJ1H-H CPU65", "cj1h"], ["CJ1M CPU21", "st-il"], ["CJ1M CPU21", "download"]]) {
      const project = path.join(tmp, model + " " + (family || "starter"));
      const args = ["omron", "new-project", "-model", model, "-timer-mode", "bcd"];
      if (family === "st-il") {
        fs.mkdirSync(project);
        for (const file of ["cj1.json", "settings.json", "program.json", "Temperature.st", "Humidity.st"]) {
          fs.copyFileSync(path.join(repo, "omron/internal/research/cpu21sample", file), path.join(project, file));
        }
      } else {
        if (family === "download") {
          args.push("-check-unit-version", "3.0", "-source", path.join(repo, "omron/commission/testdata/cj1m-new4/source.cxp"),
            "-settings", path.join(fixtures, "n1-example-settings.json"));
        } else if (family) {
          args.push("-check-unit-version", "3.0", "-source", path.join(fixtures, family + "-bool-control-unit30.cxp"),
            "-settings", path.join(fixtures, "n1-bool-control-settings.json"));
        }
        execFileSync(cli, [...args, project], {env: {...process.env, NAUTILUS_CLI: "1"}});
      }
      fs.mkdirSync(path.join(project, ".vscode"));
      fs.writeFileSync(path.join(project, ".vscode/settings.json"), JSON.stringify({"nautilus.cliPath": cli}));
      const profile = family || "starter";
      await runTests({
        vscodeExecutablePath: process.env.VSCODE_EXECUTABLE_PATH,
        extensionDevelopmentPath: extension,
        extensionTestsPath: path.join(__dirname, "cj1Suite"),
        launchArgs: [project, "--disable-extensions", "--skip-welcome", "--disable-workspace-trust",
          "--user-data-dir=" + path.join(tmp, "profile-" + profile), "--extensions-dir=" + path.join(tmp, "extensions-" + profile)],
        extensionTestsEnv: {NAUTILUS_CLI: "1", NAUTILUS_E2E_BIN: cli,
          CJ1_CONTROL_GOLDEN: family === "cj1m" || family === "cj1h" ? path.join(fixtures, family + "-f7-bool-control-unit30.n1") : "",
          CJ1_DOWNLOAD_REPO: family === "download" ? repo : ""},
      });
    }
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
  }
}

main().catch(err => { console.error(err); process.exitCode = 1; });
