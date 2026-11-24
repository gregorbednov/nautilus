package commission_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/commission"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/fins"
)

func TestControlledDownloadWithFBSource(t *testing.T) {
	read := func(path string) []byte {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	var settings compiler.N1ImageSettings
	if err := json.Unmarshal(read("../compiler/testdata/n1-example-settings.json"), &settings); err != nil {
		t.Fatal(err)
	}
	profile := codec.Profile{Model: codec.CJ1MCPU21, TimerCounterMode: codec.TimerModeBCD}
	proof, wire, err := compiler.BuildCXPImageForDownload(profile, read("testdata/cj1m-new4/source.cxp"), "auto", "Resource[0]", settings)
	if err != nil {
		t.Fatal(err)
	}
	resident := read("testdata/cj1m-new4/resident.um")
	archive := proof.SourceArchive()
	if len(archive) == 0 {
		t.Fatal("FB source archive missing from the build")
	}
	if err := json.Unmarshal(read("../compiler/testdata/n1-finalization-probe-settings.json"), &settings); err != nil {
		t.Fatal(err)
	}
	settings.DefinitionNumbers = map[string]int{}
	noFB, noFBWire, err := compiler.BuildCXPImageForDownload(profile, read("../compiler/testdata/n1-no-fb.cxt"), "auto", "Resource[0]", settings)
	if err != nil || len(noFB.SourceArchive()) != 0 {
		t.Fatalf("no-FB build: archive length %d, error %v", len(noFB.SourceArchive()), err)
	}

	for _, scenario := range []string{"run-and-return", "preexisting-archive", "retry-once", "persistent-refusal", "no-fb-replaces-archive", "inconsistent-backup", "file-refused", "file-absent-after-ack", "file-corrupt-after-ack"} {
		t.Run(scenario, func(t *testing.T) {
			p := newProgramPeer(t)
			p.cpu, p.controlEnabled = observation(t, "cj1m").CPU.Raw, true
			p.finalize = func(data []byte) {
				if len(data) == len(resident) {
					copy(data[:8], resident[:8])
					copy(data[16:20], resident[16:20])
				}
			}
			switch scenario {
			case "preexisting-archive", "no-fb-replaces-archive":
				p.archive = bytes.Repeat([]byte{0x51}, len(archive))
			case "retry-once":
				p.archive = bytes.Repeat([]byte{0x51}, len(archive))
				p.archiveRefusedWrites = []int{1, 3}
			case "persistent-refusal":
				p.archive = bytes.Repeat([]byte{0x51}, len(archive))
				p.archiveError = fins.FBSourceChangeRefused
			case "inconsistent-backup":
				p.archive = []byte{1, 2}
				p.corruptArchiveRead = 2
			case "file-refused":
				p.archiveError = 0x2108
			case "file-absent-after-ack":
				p.omitArchive = true
			case "file-corrupt-after-ack":
				p.corruptArchiveWrite = true
			}
			original := bytes.Clone(p.stored)
			originalArchive := bytes.Clone(p.archive)
			client := p.dial(t)
			defer client.Close()
			journal := &memoryJournal{}
			cfg := commission.ProgramSessionConfig{
				Model: codec.CJ1MCPU21, ExpectedPeer: p.listener.Addr().String(), ExpectedCPUData: p.cpu,
				ExpectedOriginal: original, Candidate: wire, Build: proof, BlockBytes: 996, CJ1MNew4Readback: resident,
				PreserveFBSource: true,
			}
			if scenario == "no-fb-replaces-archive" {
				cfg.Candidate, cfg.Build, cfg.CJ1MNew4Readback = noFBWire, noFB, nil
			}
			session, err := commission.NewControlledProgramSession(client, cfg, journal)
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			ctx := context.Background()
			if scenario == "inconsistent-backup" {
				if err := session.Prepare(ctx); err == nil || p.writeCount() != 0 {
					t.Fatal("inconsistent archive backup was accepted")
				}
				return
			}
			mustPrepareEnter(t, session)
			writeErr := session.WriteCandidate(ctx)
			switch scenario {
			case "file-refused", "persistent-refusal", "file-absent-after-ack", "file-corrupt-after-ack":
				if writeErr == nil || session.RequestExecution(ctx, fins.CPUModeRun) == nil || session.Snapshot().ExecutionRequested {
					t.Fatalf("execution allowed after failed transfer: %v", writeErr)
				}
				p.mu.Lock()
				defer p.mu.Unlock()
				expectedWriteCount := 1
				if scenario == "persistent-refusal" {
					expectedWriteCount = 2
				}
				if p.archiveWrites != expectedWriteCount {
					t.Fatalf("unexpected FBL write count: got %d, want %d", p.archiveWrites, expectedWriteCount)
				}
				return
			}
			if writeErr != nil {
				t.Fatal(writeErr)
			}
			if scenario != "no-fb-replaces-archive" {
				if err := session.RequestExecution(ctx, fins.CPUModeRun); err != nil {
					t.Fatal(err)
				}
				if flags, err := session.ReadExecutionTasks(ctx, 2); err != nil || !bytes.Equal(flags, []byte{1, 1}) {
					t.Fatalf("task flags: %x, %v", flags, err)
				}
			}
			p.mu.Lock()
			installed := bytes.Clone(p.archive)
			p.mu.Unlock()
			if !bytes.Equal(installed, cfg.Build.SourceArchive()) || !bytes.Equal(journal.bundle.SourceArchive, installed) {
				t.Fatal("transferred archive and journal must use the build output")
			}
			if err := session.RestoreOriginalInSession(ctx); err != nil {
				t.Fatal(err)
			}
			if err := session.FinishProgram(ctx); err != nil {
				t.Fatal(err)
			}
			fresh := p.dial(t)
			defer fresh.Close()
			if err := session.VerifyOriginalFiles(ctx, fresh); err != nil {
				t.Fatal(err)
			}
			p.mu.Lock()
			defer p.mu.Unlock()
			expectedOrder := []uint16{0x0c01, 0x0402, 0x2203, 0x0401, 0x0402, 0x0c03}
			switch scenario {
			case "preexisting-archive":
				expectedOrder = []uint16{0x0c01, 0x0402, 0x2203, 0x0401, 0x0402, 0x2203, 0x0c03}
			case "retry-once":
				expectedOrder = []uint16{0x0c01, 0x0402, 0x2203, 0x2203, 0x0401, 0x0402, 0x2203, 0x2203, 0x0c03}
			case "no-fb-replaces-archive":
				expectedOrder = []uint16{0x0c01, 0x0402, 0x2203, 0x0c03}
			}
			if !slices.Equal(p.controls, expectedOrder) || !bytes.Equal(p.stored, original) || !bytes.Equal(p.archive, originalArchive) {
				t.Fatalf("incorrect return or control order: %04x", p.controls)
			}
			if !bytes.Equal(journal.bundle.OriginalArchive, originalArchive) || session.Snapshot().OriginalArchiveRestored != (len(originalArchive) != 0) {
				t.Fatal("original archive missing from backup or restore")
			}
		})
	}
}
