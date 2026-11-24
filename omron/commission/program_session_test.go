package commission_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/commission"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/fins"
	"github.com/joyautomation/nautilus/omron/transfer"
)

// Opaque byte-store protocol peer: no CPU execution/finalization or protection.
// All connections bind loopback; tests never use the real controller addresses.
type programPeer struct {
	controlEnabled               bool
	owner                        byte
	nextNode                     byte
	controls                     []uint16
	archive                      []byte
	archiveError                 uint16
	archiveRefusedWrites         []int
	archiveWrites                int
	omitArchive                  bool
	archiveReads                 int
	corruptArchiveRead           int
	corruptArchiveWrite          bool
	dropControl                  uint16
	ignoreMode                   bool
	stayStopped                  bool
	corruptBackup                bool
	reads                        int
	mu                           sync.Mutex
	listener                     net.Listener
	cpu, status, stored, pending []byte
	writes                       int
	dropACK, corruptReadback     bool
	finalize                     func([]byte)
	wg                           sync.WaitGroup
	conns                        []net.Conn
}

func newProgramPeer(t *testing.T) *programPeer {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile("../um/testdata/cj1h-baseline.n1")
	if err != nil {
		t.Fatal(err)
	}
	p := &programPeer{listener: l, cpu: observation(t, "cj1h").CPU.Raw, status: make([]byte, 26), stored: original}
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			p.mu.Lock()
			p.conns = append(p.conns, c)
			p.mu.Unlock()
			p.wg.Add(1)
			go func() { defer p.wg.Done(); defer c.Close(); p.serve(c) }()
		}
	}()
	t.Cleanup(func() {
		l.Close()
		<-acceptDone
		p.mu.Lock()
		for _, c := range p.conns {
			c.Close()
		}
		p.mu.Unlock()
		p.wg.Wait()
	})
	return p
}
func wireRead(c net.Conn) (uint32, []byte, error) {
	h := make([]byte, 16)
	if _, err := io.ReadFull(c, h); err != nil {
		return 0, nil, err
	}
	n := binary.BigEndian.Uint32(h[4:8])
	if string(h[:4]) != "FINS" || n < 8 || n > 65536 {
		return 0, nil, errors.New("bad TCP frame")
	}
	b := make([]byte, n-8)
	_, err := io.ReadFull(c, b)
	return binary.BigEndian.Uint32(h[8:12]), b, err
}
func wireWrite(c net.Conn, cmd uint32, b []byte) error {
	f := make([]byte, 16+len(b))
	copy(f, "FINS")
	binary.BigEndian.PutUint32(f[4:], uint32(8+len(b)))
	binary.BigEndian.PutUint32(f[8:], cmd)
	copy(f[16:], b)
	for len(f) > 0 {
		n, err := c.Write(f)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		f = f[n:]
	}
	return nil
}
func (p *programPeer) serve(c net.Conn) {
	cmd, _, err := wireRead(c)
	if err != nil || cmd != 0 {
		return
	}
	p.mu.Lock()
	source := byte(10) + p.nextNode
	p.nextNode++
	p.mu.Unlock()
	if wireWrite(c, 1, []byte{0, 0, 0, source, 0, 0, 0, 20}) != nil {
		return
	}
	for {
		cmd, req, err := wireRead(c)
		if err != nil || cmd != 2 || len(req) < 12 {
			return
		}
		data, drop, err := p.answer(req)
		var end *peerEndError
		if errors.As(err, &end) {
			data = end.data
		} else if err != nil {
			return
		}
		if drop {
			return
		}
		b := make([]byte, 14+len(data))
		copy(b, req[:12])
		b[0] = 0xc0
		copy(b[3:6], req[6:9])
		copy(b[6:9], req[3:6])
		copy(b[14:], data)
		if end != nil {
			binary.BigEndian.PutUint16(b[12:], end.code)
		}
		if wireWrite(c, 2, b) != nil {
			return
		}
	}
}
func (p *programPeer) answer(req []byte) ([]byte, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch binary.BigEndian.Uint16(req[10:12]) {
	case 0x0501:
		return bytes.Clone(p.cpu), false, nil
	case 0x0601:
		return bytes.Clone(p.status), false, nil
	case 0x0101:
		if !bytes.Equal(req[12:], []byte{6, 0, 0, 0, 0, 2}) || p.status[0]&1 == 0 {
			return nil, false, errors.New("unexpected task read")
		}
		return []byte{1, 1}, false, nil
	case 0x2201:
		if !bytes.Equal(req[12:], []byte{0xc0, 0, 0, 0, 0, 0, 0, 0}) {
			return nil, false, errors.New("unexpected catalog request")
		}
		catalog := make([]byte, 28)
		copy(catalog, "            ")
		catalog[17] = 4
		if len(p.archive) != 0 {
			catalog[25], catalog[27] = 1, 1
			row := make([]byte, 22)
			copy(row, "SRCPRGRM.FBL")
			binary.BigEndian.PutUint32(row[16:], uint32(len(p.archive)))
			catalog = append(catalog, row...)
		}
		return catalog, false, nil
	case 0x2202:
		if len(req) != 34 || !bytes.Equal(req[12:26], []byte("\xc0\x01SRCPRGRM.FBL")) || binary.BigEndian.Uint16(req[32:34]) != 0 {
			return nil, false, errors.New("unexpected archive read")
		}
		offset := int(binary.BigEndian.Uint32(req[26:30]))
		count := int(binary.BigEndian.Uint16(req[30:32]))
		if offset >= len(p.archive) || count == 0 || offset+count > len(p.archive) {
			return nil, false, errors.New("archive read range")
		}
		p.archiveReads++
		data := make([]byte, 10+count)
		binary.BigEndian.PutUint32(data, uint32(len(p.archive)))
		binary.BigEndian.PutUint32(data[4:], uint32(offset))
		flags := uint16(count)
		if offset+count == len(p.archive) {
			flags |= 0x8000
		}
		binary.BigEndian.PutUint16(data[8:], flags)
		copy(data[10:], p.archive[offset:offset+count])
		if p.archiveReads == p.corruptArchiveRead {
			data[10] ^= 1
		}
		return data, false, nil
	case 0x2203:
		p.controls = append(p.controls, 0x2203)
		p.archiveWrites++
		if slices.Contains(p.archiveRefusedWrites, p.archiveWrites) {
			return nil, false, &peerEndError{fins.FBSourceChangeRefused, nil}
		}
		if p.archiveError != 0 {
			return nil, false, &peerEndError{p.archiveError, nil}
		}
		if p.owner != req[7] || p.status[1] != 0 || len(req) < 37 ||
			!bytes.Equal(req[12:32], []byte("\xc0\x01\x00\x01SRCPRGRM.FBL\x00\x00\x00\x00")) ||
			binary.BigEndian.Uint16(req[32:]) != 0x8000|uint16(len(req)-36) ||
			binary.BigEndian.Uint16(req[len(req)-2:]) != 0 {
			return nil, false, errors.New("bad archive write")
		}
		if !p.omitArchive {
			p.archive = bytes.Clone(req[34 : len(req)-2])
			if p.corruptArchiveWrite {
				p.archive[0] ^= 1
			}
		}
		return nil, false, nil
	case 0x0306:
		if len(req) != 20 {
			return nil, false, errors.New("read size")
		}
		offset := int(binary.BigEndian.Uint32(req[14:18]))
		count := int(binary.BigEndian.Uint16(req[18:20]))
		if offset >= len(p.stored) {
			return nil, false, errors.New("read offset")
		}
		if offset == 0 {
			p.reads++
		}
		count = min(count, len(p.stored)-offset)
		data := make([]byte, 8+count)
		copy(data, req[12:20])
		flags := uint16(count)
		if offset+count == len(p.stored) {
			flags |= 0x8000
		}
		binary.BigEndian.PutUint16(data[6:], flags)
		copy(data[8:], p.stored[offset:offset+count])
		if (p.corruptReadback && p.writes > 0 || p.corruptBackup && p.reads == 2) && offset == 0 {
			data[8] ^= 1
			p.corruptReadback = false
		}
		return data, false, nil
	case 0x0c01, 0x0c03, 0x0401, 0x0402:
		if !p.controlEnabled {
			return nil, false, errors.New("unexpected control")
		}
		command := binary.BigEndian.Uint16(req[10:12])
		p.controls = append(p.controls, command)
		if len(req) < 14 || !bytes.Equal(req[12:14], []byte{0xff, 0xff}) {
			return nil, false, errors.New("bad program selector")
		}
		switch command {
		case 0x0c01:
			if p.owner != 0 && p.owner != req[7] {
				return nil, false, &peerEndError{0x3001, []byte{0, p.owner, 0}}
			}
			p.owner = req[7]
		case 0x0c03:
			p.owner = 0
		case 0x0402:
			if p.owner != req[7] {
				return nil, false, &peerEndError{0x3001, nil}
			}
			if !p.ignoreMode {
				p.status[1] = 0
				p.status[0] &^= 1
			}
		case 0x0401:
			if p.owner != req[7] {
				return nil, false, &peerEndError{0x3001, nil}
			}
			if len(req) != 15 || (req[14] != 2 && req[14] != 4) {
				return nil, false, errors.New("bad execution mode")
			}
			if !p.ignoreMode {
				p.status[1] = req[14]
				if !p.stayStopped {
					p.status[0] |= 1
				}
			}
		}
		drop := p.dropControl == command
		if drop {
			p.dropControl = 0
		}
		return nil, drop, nil
	case 0x0307:
		if p.controlEnabled && (p.owner != req[7] || p.status[1] != 0) {
			return nil, false, &peerEndError{0x3001, nil}
		}
		block, err := transfer.ValidateProgramWriteRequest(req[12:])
		if err != nil {
			return nil, false, err
		}
		p.writes++
		if block.OffsetBytes == 0 {
			p.pending = nil
		}
		if int(block.OffsetBytes) != len(p.pending) {
			return nil, false, errors.New("write offset")
		}
		p.pending = append(p.pending, req[20:]...)
		if block.Final {
			p.archive = nil
			p.stored = bytes.Clone(p.pending)
			if p.finalize != nil {
				p.finalize(p.stored)
			}
		}
		data := bytes.Clone(req[12:20])
		data[6] &= 0x7f
		return data, p.dropACK, nil
	}
	return nil, false, fmt.Errorf("unexpected command %x", req[10:12])
}
func (p *programPeer) dial(t *testing.T) *fins.Client {
	t.Helper()
	c, err := fins.DialTCP(context.Background(), p.listener.Addr().String(), fins.Route{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func sessionConfig(t *testing.T, p *programPeer) commission.ProgramSessionConfig {
	return sessionConfigForModel(t, p, codec.CJ1HCPU65)
}
func sessionConfigForModel(t *testing.T, p *programPeer, model string) commission.ProgramSessionConfig {
	t.Helper()
	source, err := os.ReadFile("../compiler/testdata/n1-no-fb.cxt")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../compiler/testdata/n1-finalization-probe-settings.json")
	if err != nil {
		t.Fatal(err)
	}
	var settings compiler.N1ImageSettings
	if err = json.Unmarshal(raw, &settings); err != nil {
		t.Fatal(err)
	}
	path := "../compiler/testdata/n1-no-fb-cj1h-h.n1"
	if model == codec.CJ1MCPU21 {
		path = "../compiler/testdata/n1-no-fb-cj1m.n1"
	}
	image, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := compiler.VerifyCXPImageBuild(codec.Profile{Model: model, TimerCounterMode: codec.TimerModeBCD}, source, "auto", "Resource[0]", settings, image)
	if err != nil {
		t.Fatal(err)
	}
	return commission.ProgramSessionConfig{Model: model, ExpectedPeer: p.listener.Addr().String(), ExpectedCPUData: bytes.Clone(p.cpu), Candidate: image, Build: proof, BlockBytes: 996}
}

type memoryJournal struct {
	bundle commission.PreparedProgramBundle
	events []commission.ProgramSessionEvent
	fail   string
	mutate bool
}

func (j *memoryJournal) SavePrepared(b commission.PreparedProgramBundle) error {
	if j.fail == "save" {
		return errors.New("backup unavailable")
	}
	j.bundle = b
	if j.mutate {
		b.Candidate[0] ^= 1
		b.OriginalFirst[0] ^= 1
		b.OriginalSecond[0] ^= 1
	}
	return nil
}
func (j *memoryJournal) Append(e commission.ProgramSessionEvent) error {
	if e.Kind == j.fail {
		return errors.New("journal unavailable")
	}
	j.events = append(j.events, e)
	return nil
}
func newSession(t *testing.T, p *programPeer, j commission.ProgramSessionJournal) *commission.ProgramSession {
	t.Helper()
	c := p.dial(t)
	s, err := commission.NewProgramSession(c, sessionConfig(t, p), j)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(c.Close(), fins.ErrClientTransferred) {
		t.Fatal("old client still owns connection")
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func TestProgramSessionCandidateAndExplicitOriginalReturn(t *testing.T) {
	p := newProgramPeer(t)
	original := p.programBytes()
	dir := filepath.Join(t.TempDir(), "trial")
	j, err := commission.NewFileProgramJournal(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := newSession(t, p, j)
	ctx := context.Background()
	if err = s.WriteCandidate(ctx); err == nil {
		t.Fatal("write without prepare")
	}
	if err = s.Prepare(ctx); err != nil {
		t.Fatal(err)
	}
	if p.writeCount() != 0 {
		t.Fatal("prepare wrote program")
	}
	if err = s.WriteCandidate(ctx); err != nil {
		t.Fatal(err)
	}
	snap := s.Snapshot()
	if snap.State != "candidate-matched" || snap.Candidate.AcknowledgedBlocks != 3 || snap.ProgramBytesReconciled {
		t.Fatal(snap)
	}
	if err = s.WriteCandidate(ctx); err == nil {
		t.Fatal("candidate repeated")
	}
	if err = s.ReturnOriginal(ctx, p.dial(t)); err != nil {
		t.Fatal(err)
	}
	snap = s.Snapshot()
	if snap.State != "original-matched" || !snap.ProgramBytesReconciled || snap.OriginalReturn.AcknowledgedBlocks != 5 || snap.HardwareQualified || snap.WriteQualified || snap.RestoreQualified || snap.FullControllerBackup {
		t.Fatal(snap)
	}
	if !bytes.Equal(p.programBytes(), original) || p.writeCount() != 8 {
		t.Fatal("original return not exact")
	}

	for _, name := range []string{"original-1.um", "original-2.um"} {
		saved, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || !bytes.Equal(saved, original) {
			t.Fatal("saved original mismatch", name, err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var candidateReadback, originalReadback, attempts int
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
		var event commission.ProgramSessionEvent
		if err = json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		if event.Kind == "write-attempt" {
			attempts++
		}
		if event.Kind == "readback-observed" {
			switch event.Phase {
			case "candidate":
				if !bytes.Equal(event.ReadbackData, candidate(t)) {
					t.Fatal("candidate readback not retained")
				}
				candidateReadback++
			case "original-return":
				if !bytes.Equal(event.ReadbackData, original) {
					t.Fatal("original readback not retained")
				}
				originalReadback++
			}
		}
	}
	if attempts != 8 || candidateReadback != 1 || originalReadback != 1 {
		t.Fatal("incomplete journal")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if !s.Snapshot().ProgramBytesReconciled {
		t.Fatal("close erased byte reconciliation")
	}
	fresh := p.dial(t)
	defer fresh.Close()
	if s.ReturnOriginal(ctx, fresh) == nil {
		t.Fatal("return repeated")
	}
}

func TestNew4ResidentReference(t *testing.T) {
	read := func(path string) []byte {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	resident := read("testdata/cj1m-new4/resident.um")
	wire := bytes.Clone(resident)
	clear(wire[:8])
	clear(wire[16:20])
	var settings compiler.N1ImageSettings
	if err := json.Unmarshal(read("../compiler/testdata/n1-example-settings.json"), &settings); err != nil {
		t.Fatal(err)
	}
	proof, err := compiler.VerifyCXPImageBuild(codec.Profile{Model: codec.CJ1MCPU21, TimerCounterMode: codec.TimerModeBCD}, read("testdata/cj1m-new4/source.cxp"), "auto", "Resource[0]", settings, wire)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"valid", "reference-corrupt", "wire-corrupt", "readback-corrupt", "before-run-corrupt", "default-strict"} {
		t.Run(scenario, func(t *testing.T) {
			p := newProgramPeer(t)
			p.cpu = observation(t, "cj1m").CPU.Raw
			p.controlEnabled = true
			p.stored = bytes.Clone(resident)
			p.finalize = func(data []byte) {
				copy(data[:8], resident[:8])
				copy(data[16:20], resident[16:20])
			}
			cfg := commission.ProgramSessionConfig{Model: codec.CJ1MCPU21, ExpectedPeer: p.listener.Addr().String(), ExpectedCPUData: p.cpu, Candidate: bytes.Clone(wire), Build: proof, BlockBytes: 996, CJ1MNew4Readback: bytes.Clone(resident)}
			switch scenario {
			case "reference-corrupt":
				cfg.CJ1MNew4Readback[16] ^= 1
			case "wire-corrupt":
				cfg.Candidate[42] ^= 1
			case "default-strict":
				cfg.CJ1MNew4Readback = nil
			case "readback-corrupt":
				p.corruptReadback = true
			}
			c := p.dial(t)
			defer c.Close()
			j := &memoryJournal{}
			s, err := commission.NewControlledProgramSession(c, cfg, j)
			if scenario == "reference-corrupt" || scenario == "wire-corrupt" {
				if err == nil || p.writeCount() != 0 {
					t.Fatal("invalid reference pair accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			clear(cfg.CJ1MNew4Readback)
			mustPrepareEnter(t, s)
			ctx := context.Background()
			err = s.WriteCandidate(ctx)
			p.mu.Lock()
			wireMatches := p.writes == 5 && bytes.Equal(p.pending, wire)
			p.mu.Unlock()
			if !wireMatches {
				t.Fatal("wire image mismatch")
			}
			if scenario == "default-strict" || scenario == "readback-corrupt" {
				if err == nil || s.Snapshot().State != "stopped" {
					t.Fatal("readback mismatch accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			snapshot := s.Snapshot().Candidate
			if snapshot.State != transfer.ProgramAuditResidentBytesMatched || snapshot.UncertainWrite || !snapshot.Readback.Exact || j.bundle.ResidentReferenceSHA256 != snapshot.ResidentReferenceSHA256 {
				t.Fatal(snapshot)
			}
			if scenario == "before-run-corrupt" {
				p.mu.Lock()
				p.stored[42] ^= 1
				p.mu.Unlock()
			}
			err = s.RequestExecution(ctx, fins.CPUModeRun)
			if scenario == "before-run-corrupt" {
				if err == nil || s.Snapshot().ExecutionRequested {
					t.Fatal("changed image accepted for RUN")
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestProgramSessionStopsBeforeWriteOnGuardsAndJournalFailure(t *testing.T) {
	for _, name := range []string{"save", "prepared", "write-attempt", "RUN", "fatal", "nonfatal", "identity", "pair", "changed", "cancel"} {
		t.Run(name, func(t *testing.T) {
			p := newProgramPeer(t)
			j := &memoryJournal{fail: name}
			s := newSession(t, p, j)
			ctx := context.Background()
			p.mu.Lock()
			switch name {
			case "RUN":
				p.status[1] = 4
			case "fatal":
				p.status[3] = 1
			case "nonfatal":
				p.status[5] = 1
			case "identity":
				p.cpu[len(p.cpu)-1] ^= 1
			case "pair":
				p.corruptBackup = true
			}
			p.mu.Unlock()
			err := s.Prepare(ctx)
			if name == "pair" {
				if err == nil {
					t.Fatal("inconsistent backup accepted")
				}
			}
			if err == nil {
				if name == "changed" {
					p.mu.Lock()
					p.stored[0] ^= 1
					p.mu.Unlock()
				}
				if name == "cancel" {
					cancelled, cancel := context.WithCancel(ctx)
					cancel()
					ctx = cancelled
				}
				err = s.WriteCandidate(ctx)
			}
			if err == nil || s.Snapshot().State != "stopped" || p.writeCount() != 0 {
				t.Fatal("guard failed", err, s.Snapshot(), p.writeCount())
			}
			if s.WriteCandidate(context.Background()) == nil {
				t.Fatal("stopped session resumed")
			}
			fresh := p.dial(t)
			defer fresh.Close()
			if s.ReturnOriginal(context.Background(), fresh) == nil {
				t.Fatal("return allowed before candidate attempt")
			}
		})
	}
}
func TestProgramSessionFailedAttemptNeverAutoReturns(t *testing.T) {
	for _, name := range []string{"lost-ack", "ack", "readback", "readback-observed", "service-header"} {
		t.Run(name, func(t *testing.T) {
			p := newProgramPeer(t)
			j := &memoryJournal{}
			s := newSession(t, p, j)
			if err := s.Prepare(context.Background()); err != nil {
				t.Fatal(err)
			}
			p.mu.Lock()
			if name == "lost-ack" {
				p.dropACK = true
			}
			if name == "service-header" {
				p.corruptReadback = true
			}
			p.mu.Unlock()
			if name == "ack" || name == "readback" || name == "readback-observed" {
				j.fail = name
			}
			if err := s.WriteCandidate(context.Background()); err == nil {
				t.Fatal("failure ignored")
			}
			snap := s.Snapshot()
			writes := p.writeCount()
			if snap.State != "stopped" || writes == 0 || snap.ReturnAttempted || snap.ProgramBytesReconciled {
				t.Fatal(snap)
			}
			if (name == "lost-ack" || name == "ack") && writes != 1 {
				t.Fatal("lost ACK retried")
			}
			if s.WriteCandidate(context.Background()) == nil || p.writeCount() != writes {
				t.Fatal("candidate resumed")
			}
			if name == "service-header" && (snap.Candidate.Readback == nil || !snap.Candidate.Readback.OnlyServiceWords014Differ) {
				t.Fatal("service-only mismatch lost", snap)
			}
			j.fail = ""
			p.mu.Lock()
			p.dropACK = false
			p.mu.Unlock()
			if err := s.ReturnOriginal(context.Background(), p.dial(t)); err != nil {
				t.Fatal(err)
			}
			snap = s.Snapshot()
			if !snap.ProgramBytesReconciled || snap.Candidate.State != transfer.ProgramAuditStopped {
				t.Fatal("old audit revived", snap)
			}
		})
	}
}
func TestProgramSessionPrivateBytesAndReturnBinding(t *testing.T) {
	p := newProgramPeer(t)
	original := p.programBytes()
	j := &memoryJournal{mutate: true}
	c := p.dial(t)
	cfg := sessionConfig(t, p)
	s, err := commission.NewProgramSession(c, cfg, j)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cfg.Candidate[0] ^= 1
	cfg.ExpectedCPUData[0] ^= 1
	if err = s.Prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = s.WriteCandidate(context.Background()); err != nil {
		t.Fatal(err)
	}
	other := newProgramPeer(t)
	foreign := other.dial(t)
	defer foreign.Close()
	if s.ReturnOriginal(context.Background(), foreign) == nil || other.writeCount() != 0 || s.Snapshot().ReturnAttempted {
		t.Fatal("foreign return accepted")
	}
	if err = s.ReturnOriginal(context.Background(), p.dial(t)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(p.programBytes(), original) {
		t.Fatal("journal/input mutated private original")
	}
}
func TestFileProgramJournalExclusiveAndFailureLatch(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "journal")
	j, err := commission.NewFileProgramJournal(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = commission.NewFileProgramJournal(dir); err == nil {
		t.Fatal("existing journal overwritten")
	}
	if err = j.Append(commission.ProgramSessionEvent{Kind: "write-attempt"}); err == nil {
		t.Fatal("attempt before backup")
	}
	bundle := commission.PreparedProgramBundle{Candidate: []byte("candidate"), OriginalFirst: []byte("original"), OriginalSecond: []byte("original")}
	if err = j.SavePrepared(bundle); err != nil {
		t.Fatal(err)
	}
	if err = j.SavePrepared(bundle); err == nil {
		t.Fatal("backup overwritten")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "prepared.json"))
	if err != nil || !bytes.Contains(raw, []byte(`"fullControllerBackup": false`)) {
		t.Fatal("missing manifest", err)
	}
	if err = os.Remove(filepath.Join(dir, "events.jsonl")); err != nil {
		t.Fatal(err)
	}
	if j.Append(commission.ProgramSessionEvent{Kind: "write-attempt"}) == nil {
		t.Fatal("journal recreated")
	}
	if err = os.WriteFile(filepath.Join(dir, "events.jsonl"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if j.Append(commission.ProgramSessionEvent{Kind: "write-attempt"}) == nil {
		t.Fatal("failed journal resumed")
	}
}

func TestProgramSessionReturnFailureIsOneShot(t *testing.T) {
	for _, fault := range []string{"mode", "identity", "lost-ack"} {
		t.Run(fault, func(t *testing.T) {
			p := newProgramPeer(t)
			s := newSession(t, p, &memoryJournal{})
			if err := s.Prepare(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err := s.WriteCandidate(context.Background()); err != nil {
				t.Fatal(err)
			}
			p.mu.Lock()
			before := p.writes
			switch fault {
			case "mode":
				p.status[1] = 4
			case "identity":
				p.cpu[len(p.cpu)-1] ^= 1
			case "lost-ack":
				p.dropACK = true
			}
			p.mu.Unlock()
			if s.ReturnOriginal(context.Background(), p.dial(t)) == nil {
				t.Fatal("return failure ignored")
			}
			snap := s.Snapshot()
			if snap.State != "stopped" || !snap.ReturnAttempted || snap.ProgramBytesReconciled {
				t.Fatal(snap)
			}
			if fault == "lost-ack" {
				before++
			}
			if p.writeCount() != before {
				t.Fatal("unexpected return writes", p.writeCount(), before)
			}
			fresh := p.dial(t)
			defer fresh.Close()
			if s.ReturnOriginal(context.Background(), fresh) == nil || p.writeCount() != before {
				t.Fatal("return resumed")
			}
		})
	}
}
func TestProgramSessionRejectsInvalidConfigurationWithoutRetiringClient(t *testing.T) {
	for _, fault := range []string{"proof", "candidate", "model", "peer", "CPU", "capacity", "block"} {
		t.Run(fault, func(t *testing.T) {
			p := newProgramPeer(t)
			c := p.dial(t)
			defer c.Close()
			cfg := sessionConfig(t, p)
			switch fault {
			case "proof":
				cfg.Build = nil
			case "candidate":
				cfg.Candidate[0] ^= 1
			case "model":
				cfg.Model = codec.CJ1MCPU21
			case "peer":
				cfg.ExpectedPeer = "127.0.0.1:1"
			case "CPU":
				cfg.ExpectedCPUData = []byte{0}
			case "capacity":
				cfg.ExpectedCPUData[80] = 0
				cfg.ExpectedCPUData[81] = 1
			case "block":
				cfg.BlockBytes = 1000
			}
			if _, err := commission.NewProgramSession(c, cfg, &memoryJournal{}); err == nil {
				t.Fatal("invalid config accepted")
			}
			if _, err := c.Status(context.Background()); err != nil {
				t.Fatal("invalid config consumed client", err)
			}
			if p.writeCount() != 0 {
				t.Fatal("configuration wrote program")
			}
		})
	}
}

func (p *programPeer) writeCount() int { p.mu.Lock(); defer p.mu.Unlock(); return p.writes }
func (p *programPeer) programBytes() []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	return bytes.Clone(p.stored)
}

func TestFileProgramJournalPreservesPartialBackup(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "partial")
	j, err := commission.NewFileProgramJournal(dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "candidate.n1")
	if err = os.WriteFile(path, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	if j.SavePrepared(commission.PreparedProgramBundle{Candidate: []byte("replacement")}) == nil {
		t.Fatal("collision overwritten")
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "existing" {
		t.Fatal("partial backup changed", err)
	}
	if j.Append(commission.ProgramSessionEvent{Kind: "write-attempt"}) == nil {
		t.Fatal("partial journal accepted attempt")
	}
	if _, err = os.Stat(filepath.Join(dir, "prepared.json")); !os.IsNotExist(err) {
		t.Fatal("partial backup marked prepared", err)
	}
}

type peerEndError struct {
	code uint16
	data []byte
}

func (e *peerEndError) Error() string { return fmt.Sprintf("peer end code %04x", e.code) }
