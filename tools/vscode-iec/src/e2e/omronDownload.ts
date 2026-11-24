import * as assert from "node:assert/strict";
import { createHash } from "node:crypto";
import * as fs from "node:fs";
import { AddressInfo, createServer, Socket } from "node:net";
import * as path from "node:path";
import * as vscode from "vscode";
import { downloadCJ1Project } from "../omronBuild";

function tcpFrame(command: number, body: Buffer): Buffer {
  const header = Buffer.alloc(16);
  header.write("FINS");
  header.writeUInt32BE(8 + body.length, 4);
  header.writeUInt32BE(command, 8);
  return Buffer.concat([header, body]);
}

// A loopback peer uses the real FINS/TCP client and the two known image
// layouts. It checks sequencing and keeps no connection to physical hardware.
async function mockCJ1(cpu: Buffer, original: Buffer, resident: Buffer) {
  const sockets = new Set<Socket>();
  const errors: unknown[] = [];
  const wire = Buffer.from(resident);
  wire.fill(0, 0, 8).fill(0, 16, 20);
  let node = 10;
  let owner = 0;
  let mode = 0;
  let stored = original;
  let pending = Buffer.alloc(0);
  let archive = Buffer.alloc(0);
  const writes: string[] = [];

  function respond(request: Buffer): Buffer {
    const command = request.readUInt16BE(10);
    const data = request.subarray(12);
    const source = request[7];
    switch (command) {
      case 0x0501:
        return cpu;
      case 0x0601: {
        const status = Buffer.alloc(26);
        status[0] = mode === 4 ? 5 : 4;
        status[1] = mode;
        return status;
      }
      case 0x0c01:
        assert.equal(owner, 0);
        owner = source;
        return Buffer.alloc(0);
      case 0x0c03:
        assert.equal(owner, source);
        owner = 0;
        return Buffer.alloc(0);
      case 0x0402:
        assert.equal(owner, source);
        mode = 0;
        return Buffer.alloc(0);
      case 0x0401:
        assert.equal(owner, source);
        assert.deepEqual(data, Buffer.from("ffff04", "hex"));
        assert.deepEqual(stored, resident);
        assert.ok(archive.length);
        writes.push("RUN");
        mode = 4;
        return Buffer.alloc(0);
      case 0x0101:
        assert.equal(mode, 4);
        assert.deepEqual(data, Buffer.from("060000000002", "hex"));
        return Buffer.from([1, 1]);
      case 0x0306: {
        const offset = data.readUInt32BE(2);
        const count = Math.min(data.readUInt16BE(6), stored.length - offset);
        assert.ok(count > 0);
        const header = Buffer.from(data);
        header.writeUInt16BE(count | (offset + count === stored.length ? 0x8000 : 0), 6);
        return Buffer.concat([header, stored.subarray(offset, offset + count)]);
      }
      case 0x0307: {
        assert.equal(owner, source);
        assert.equal(mode, 0);
        const offset = data.readUInt32BE(2);
        const flags = data.readUInt16BE(6);
        const count = flags & 0x7fff;
        assert.equal(data.length, 8 + count);
        if (offset === 0) pending = Buffer.alloc(0);
        assert.equal(offset, pending.length);
        pending = Buffer.concat([pending, data.subarray(8)]);
        if (flags & 0x8000) {
          if (pending.equals(wire)) {
            stored = resident;
            writes.push("new4");
          } else {
            assert.deepEqual(pending, original);
            stored = original;
            writes.push("original");
          }
          archive = Buffer.alloc(0);
        }
        const ack = Buffer.from(data.subarray(0, 8));
        ack.writeUInt16BE(count, 6);
        return ack;
      }
      case 0x2201: {
        const catalog = Buffer.alloc(archive.length ? 50 : 28);
        catalog.fill(32, 0, 12);
        catalog[17] = 4;
        if (archive.length) {
          catalog[25] = catalog[27] = 1;
          catalog.write("SRCPRGRM.FBL", 28);
          catalog.writeUInt32BE(archive.length, 44);
        }
        return catalog;
      }
      case 0x2202: {
        const offset = data.readUInt32BE(14);
        const count = data.readUInt16BE(18);
        assert.ok(count > 0 && offset + count <= archive.length);
        const header = Buffer.alloc(10);
        header.writeUInt32BE(archive.length);
        header.writeUInt32BE(offset, 4);
        header.writeUInt16BE(count | (offset + count === archive.length ? 0x8000 : 0), 8);
        return Buffer.concat([header, archive.subarray(offset, offset + count)]);
      }
      case 0x2203:
        assert.equal(owner, source);
        assert.equal(mode, 0);
        assert.deepEqual(stored, resident);
        assert.equal(data.subarray(4, 16).toString(), "SRCPRGRM.FBL");
        archive = Buffer.from(data.subarray(22, -2));
        assert.equal(createHash("sha256").update(archive).digest("hex"),
          "698fbb443b7030c26a7ce4647a66ebb7479738d98faccc294c26ca6c5ef4408d");
        writes.push("FBL");
        return Buffer.alloc(0);
      default:
        throw new Error("Unexpected FINS command " + command.toString(16));
    }
  }

  const server = createServer(socket => {
    sockets.add(socket);
    socket.on("close", () => sockets.delete(socket));
    socket.on("error", error => errors.push(error));
    let buffered = Buffer.alloc(0);
    socket.on("data", chunk => {
      try {
        buffered = Buffer.concat([buffered, chunk]);
        while (buffered.length >= 16) {
          assert.equal(buffered.subarray(0, 4).toString(), "FINS");
          const length = buffered.readUInt32BE(4) + 8;
          if (buffered.length < length) return;
          const frame = buffered.subarray(0, length);
          buffered = buffered.subarray(length);
          if (frame.readUInt32BE(8) === 0) {
            socket.write(tcpFrame(1, Buffer.from([0, 0, 0, node++, 0, 0, 0, 20])));
            continue;
          }
          assert.equal(frame.readUInt32BE(8), 2);
          const request = frame.subarray(16);
          const reply = Buffer.alloc(14);
          request.copy(reply, 0, 0, 12);
          reply[0] |= 0x40;
          request.copy(reply, 3, 6, 9);
          request.copy(reply, 6, 3, 6);
          socket.write(tcpFrame(2, Buffer.concat([reply, respond(request)])));
        }
      } catch (error) {
        errors.push(error);
        socket.destroy();
      }
    });
  });
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject).listen(0, "127.0.0.1", resolve);
  });
  return {
    endpoint: `127.0.0.1:${(server.address() as AddressInfo).port}`,
    writes, errors,
    verifyOriginal() {
      assert.equal(mode, 0);
      assert.equal(owner, 0);
      assert.deepEqual(stored, original);
      assert.equal(archive.length, 0);
    },
    close() {
      for (const socket of sockets) socket.destroy();
      return new Promise<void>(resolve => server.close(() => resolve()));
    },
  };
}

export async function testCJ1Download(root: vscode.WorkspaceFolder, repo: string): Promise<void> {
  const extension = vscode.extensions.getExtension("joyauto.vscode-iec");
  assert.ok(extension);
  await extension.activate();
  assert.ok((await vscode.commands.getCommands(true)).includes("nautilus.omron.downloadProject"));
  const residentPath = path.join(repo, "omron/commission/testdata/cj1m-new4/resident.um");
  const resident = fs.readFileSync(residentPath);
  const originalPath = path.join(repo, "omron/compiler/testdata/n1-no-fb-cj1m.n1");
  const original = fs.readFileSync(originalPath);
  const status = JSON.parse(fs.readFileSync(path.join(repo, "omron/commission/testdata/bench-state-20261121/cj1m/before.json"), "utf8"));
  const plc = await mockCJ1(Buffer.from(status.cpu.raw, "base64"), original, resident);
  const out = vscode.window.createOutputChannel("Nautilus CJ1 Test");
  const folder = path.join(root.uri.fsPath, "build");
  const prompts = (confirmed: boolean) => ({
    async showQuickPick(items: readonly string[]) { return items[1]; },
    async showInputBox(options: vscode.InputBoxOptions) {
      const answer = options.title === "Omron CJ1 Target" ? plc.endpoint : "2";
      assert.equal(await options.validateInput!(answer), undefined);
      return answer;
    },
    async showOpenDialog(options: vscode.OpenDialogOptions) {
      return [vscode.Uri.file(options.title?.includes("resident") ? residentPath : originalPath)];
    },
    async showWarningMessage(message: string, options: vscode.MessageOptions, button: string) {
      assert.equal(message, "Run Test and Restore?");
      assert.equal(options.modal, true);
      assert.ok(options.detail?.includes(plc.endpoint));
      assert.ok(options.detail?.includes(vscode.Uri.file(originalPath).fsPath));
      return confirmed ? button : undefined;
    },
  });
  try {
    await downloadCJ1Project(root, out, prompts(false));
    assert.deepEqual(plc.writes, []);
    assert.ok(!fs.existsSync(folder));
    const source = await vscode.workspace.openTextDocument(vscode.Uri.joinPath(root.uri, "program.cxt"));
    const edit = new vscode.WorkspaceEdit();
    edit.insert(source.uri, source.positionAt(source.getText().length), "\n");
    assert.ok(await vscode.workspace.applyEdit(edit));
    assert.ok(source.isDirty);
    await downloadCJ1Project(root, out, prompts(true));
    assert.ok(!source.isDirty);
    assert.deepEqual(plc.errors, []);
    assert.deepEqual(plc.writes, ["new4", "FBL", "RUN", "original"]);
    plc.verifyOriginal();
    const jobs = fs.readdirSync(folder);
    assert.equal(jobs.length, 1);
    const job = path.join(folder, jobs[0]);
    assert.equal(fs.readFileSync(path.join(job, "program.cxt"), "utf8"), source.getText());
    const result = JSON.parse(fs.readFileSync(path.join(job, "result.json"), "utf8"));
    assert.equal(result.taskFlags, "0101");
    assert.equal(result.session.archiveTransferred, true);
    assert.equal(result.session.observedExecutionMode, 4);
    assert.equal(result.originalVerified, true);
    assert.deepEqual(fs.readFileSync(path.join(job, "journal/original-1.um")), original);
  } finally {
    out.dispose();
    await plc.close();
  }
}
