import * as assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import * as fs from "node:fs";
import * as path from "node:path";
import { createHash } from "node:crypto";
import * as vscode from "vscode";
import { testCJ1Download } from "./omronDownload";

async function testSTILBuild(root: string): Promise<void> {
  const build = () => vscode.commands.executeCommand("nautilus.omron.buildProject");
  const originalHash = "3b70cab1f76a274072b0b82cc88dfa9cda37ffdd39fdc3ff4855b1b8794aa6d7";
  const artifact = (hash: string) => path.join(root, "build", hash + ".n1");
  await build();
  const expected = fs.readFileSync(artifact(originalHash));
  assert.equal(expected.length, 4672);

  // The selected format and the ST dependency list both change in dirty buffers.
  const manifestPath = path.join(root, "cj1.json");
  const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
  delete manifest.sourceFormat;
  fs.writeFileSync(manifestPath, JSON.stringify(manifest));
  const programPath = path.join(root, "program.json");
  const program = fs.readFileSync(programPath, "utf8");
  fs.writeFileSync(programPath, program.replace(/Humidity/g, "Unused"));
  fs.writeFileSync(path.join(root, "Unused.st"), "");
  const edits = new vscode.WorkspaceEdit();
  const dirty: vscode.TextDocument[] = [];
  async function replace(file: string, change: (before: string) => string) {
    const doc = await vscode.workspace.openTextDocument(path.join(root, file));
    const before = doc.getText();
    const after = change(before);
    assert.notEqual(after, before);
    edits.replace(doc.uri, new vscode.Range(doc.positionAt(0), doc.positionAt(before.length)), after);
    dirty.push(doc);
    return doc;
  }
  await replace("cj1.json", () => JSON.stringify({...manifest, sourceFormat: "st-il"}));
  await replace("program.json", () => program.replace("CALL 6 2097 D100", "CALL 6 2097 D101"));
  await replace("settings.json", s => s + "\n");
  await replace("Humidity.st", s => s.replace("95.0", "96.0"));
  await replace("Temperature.st", s => s.replace("90.0", "91.0"));
  const unused = await replace("Unused.st", () => "unsaved");
  assert.ok(await vscode.workspace.applyEdit(edits));
  assert.ok(dirty.every(doc => doc.isDirty));
  for (const [from, to] of [["5c010064", "5c010065"], ["614042be", "614042c0"], ["614042b4", "614042b6"]]) {
    const word = Buffer.from(from, "hex");
    const at = expected.indexOf(word);
    assert.ok(at >= 0 && at === expected.lastIndexOf(word));
    Buffer.from(to, "hex").copy(expected, at);
  }
  const changedHash = createHash("sha256").update(expected).digest("hex");
  await build();
  assert.deepEqual(fs.readFileSync(artifact(changedHash)), expected);
  assert.ok(dirty.every(doc => doc.isDirty === (doc === unused)));
  assert.equal(fs.readFileSync(unused.fileName, "utf8"), "");
  await unused.save();
}

export async function run(): Promise<void> {
  const folder = vscode.workspace.workspaceFolders?.[0];
  const root = folder?.uri.fsPath;
  const cli = process.env.NAUTILUS_E2E_BIN;
  assert.ok(folder && root && cli);
  if (process.env.CJ1_DOWNLOAD_REPO) {
    await testCJ1Download(folder, process.env.CJ1_DOWNLOAD_REPO);
    return;
  }
  if (fs.existsSync(path.join(root, "program.json"))) {
    await testSTILBuild(root);
    return;
  }
  const source = path.join(root, "program.cxt");
  const doc = await vscode.workspace.openTextDocument(source);
  if (process.env.CJ1_CONTROL_GOLDEN) {
    const original = fs.readFileSync(process.env.CJ1_CONTROL_GOLDEN);
    const checkBuild = async (expected: Buffer) => {
      const hash = createHash("sha256").update(expected).digest("hex");
      await vscode.commands.executeCommand("nautilus.omron.buildProject");
      assert.deepEqual(fs.readFileSync(path.join(root, "build", hash + ".n1")), expected);
    };
    await checkBuild(original);
    const text = doc.getText();
    const index = text.indexOf("W100.02");
    assert.ok(index >= 0 && index === text.lastIndexOf("W100.02"));
    const edit = new vscode.WorkspaceEdit();
    edit.replace(doc.uri, new vscode.Range(doc.positionAt(index), doc.positionAt(index + 7)), "W100.03");
    assert.ok(await vscode.workspace.applyEdit(edit));
    assert.ok(doc.isDirty);
    const expected = Buffer.from(original);
    const operand = Buffer.from("560de642", "hex");
    const offset = expected.indexOf(operand);
    assert.ok(offset >= 0 && offset === expected.lastIndexOf(operand));
    expected[offset + 3] = 0x43;
    await checkBuild(expected);
    assert.ok(!doc.isDirty);
    return;
  }
  const build = async (name: string) => {
    await vscode.commands.executeCommand("nautilus.omron.buildProject");
    const candidate = path.join(root, name + ".n1");
    const report = JSON.parse(execFileSync(cli, ["omron", "compile-image", "-model", "CJ1M CPU21",
      "-resource", "Resource[0]", "-timer-mode", "bcd", "-settings", path.join(root, "settings.json"),
      "-o", candidate, source], {encoding: "utf8"}));
    const built = path.join(root, "build", report.image.sha256 + ".n1");
    assert.deepEqual(fs.readFileSync(built), fs.readFileSync(candidate));
    return report.image.sha256;
  };
  const original = await build("original");
  const notesPath = path.join(root, "notes.txt");
  fs.writeFileSync(notesPath, "");
  const notes = await vscode.workspace.openTextDocument(notesPath);
  const edit = new vscode.WorkspaceEdit();
  const text = doc.getText();
  const originalRung = 'SL:="END"';
  const pos = text.indexOf(originalRung);
  assert.ok(pos >= 0);
  const rung = new vscode.Range(doc.positionAt(pos), doc.positionAt(pos + originalRung.length));
  edit.replace(doc.uri, rung, 'SL:="NOP\nEND"');
  edit.insert(notes.uri, new vscode.Position(0, 0), "unsaved notes");
  assert.ok(await vscode.workspace.applyEdit(edit));
  assert.ok(doc.isDirty);
  const changed = await build("edited");
  assert.ok(!doc.isDirty);
  assert.ok(notes.isDirty);
  assert.equal(fs.readFileSync(notesPath, "utf8"), "");
  await notes.save();
  assert.notEqual(changed, original);
}
