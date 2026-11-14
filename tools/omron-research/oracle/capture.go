// Research-only verification of saved observations. No vendor tools or PLCs.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const compilerHash = "a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141"
const maxLogSize = 32 << 20
const maxImageSize = 4 << 20

func digest(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

type profile struct {
	Descriptor string `json:"descriptor"`
	Selector   uint32 `json:"selector"`
	Low        uint32 `json:"familyMaskLow"`
	High       uint32 `json:"familyMaskHigh"`
}
type event struct {
	Schema          int      `json:"schemaVersion"`
	Sequence        *uint64  `json:"sequence"`
	Kind            string   `json:"kind"`
	PID             int      `json:"pid"`
	Thread          int      `json:"thread"`
	Profile         *profile `json:"profile"`
	Device          string   `json:"device"`
	CPU             string   `json:"cpu"`
	Name            string   `json:"name"`
	Arch            string   `json:"arch"`
	ExpectedHash    string   `json:"expectedSHA256"`
	Result          *int     `json:"result"`
	CompileSequence *uint64  `json:"compileSequence"`
	ByteLength      int      `json:"byteLength"`
	FileHex         string   `json:"fileHex"`
	Dropped         *uint64  `json:"droppedEvents"`
}
type compilation struct {
	Sequence    uint64  `json:"sequence"`
	Thread      int     `json:"thread"`
	Profile     profile `json:"profile"`
	ObservedCPU string  `json:"observedCPU,omitempty"`
	Result      *int    `json:"result"`
	FileSHA256  string  `json:"fileSHA256,omitempty"`
	Correlation string  `json:"correlation"`
}
type captureReport struct {
	Schema                 int            `json:"schemaVersion"`
	Complete               bool           `json:"complete"`
	EventsSHA256           string         `json:"eventsSHA256"`
	EventCount             int            `json:"eventCount"`
	Compilations           []*compilation `json:"compilations"`
	Problems               []string       `json:"problems"`
	SourceIdentityVerified bool           `json:"sourceIdentityInCompilerVerified"`
	HardwareQualified      bool           `json:"hardwareQualified"`
}

func readBounded(reader io.Reader, limit int) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, fmt.Errorf("input exceeds %d bytes", limit)
	}
	return data, nil
}

// auditCapture checks the event transcript, not the honesty of its producer.
// Source identity remains unverified even for an internally complete capture.
func auditCapture(reader io.Reader, stderr string) (captureReport, error) {
	report := captureReport{Schema: 1, Compilations: []*compilation{}, Problems: []string{}}
	data, err := readBounded(reader, maxLogSize)
	if err != nil {
		return report, err
	}
	report.EventsSHA256 = digest(data)
	problem := func(message string) {
		if len(report.Problems) < 24 {
			report.Problems = append(report.Problems, message)
		}
	}
	if strings.TrimSpace(stderr) != "" {
		problem("injector stderr is not empty")
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), 2*maxImageSize+65536)
	active := map[int][]*compilation{}
	finished := map[int]*compilation{}
	resolved := map[string]struct {
		p    profile
		name string
	}{}
	pid := 0
	modules, ready, started, ended := 0, 0, 0, 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "CJ1_ORACLE ") {
			problem("unexpected injector stdout")
			continue
		}
		var entry event
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "CJ1_ORACLE ")), &entry); err != nil {
			problem("invalid event JSON: " + err.Error())
			continue
		}
		if entry.Schema != 1 || entry.Sequence == nil || entry.Kind == "" || entry.PID <= 0 || entry.Thread <= 0 {
			problem("invalid event envelope")
			continue
		}
		if ended > 0 {
			problem("event after capture-ended")
		}
		if *entry.Sequence != uint64(report.EventCount) {
			problem("nonconsecutive event sequence")
		}
		report.EventCount++
		if report.EventCount > 25000 {
			return report, fmt.Errorf("event count limit")
		}
		if pid == 0 {
			pid = entry.PID
		}
		if entry.PID != pid {
			problem("process changed within capture")
		}
		switch entry.Kind {
		case "module":
			modules++
			if modules != 1 || ready != 0 || !strings.EqualFold(entry.Name, "CX-Pdata.dll") || entry.Arch != "ia32" || entry.ExpectedHash != compilerHash {
				problem("unreviewed or out-of-order compiler module")
			}
		case "ready":
			ready++
			if modules != 1 || ready != 1 {
				problem("missing module or duplicate ready")
			}
		case "observer-started":
			started++
			if started != 1 {
				problem("duplicate observer-started")
			}
		case "cpu-profile":
			if entry.Profile == nil || entry.Profile.Descriptor == "" || entry.Profile.Selector == 0 || entry.Device == "" || entry.CPU == "" {
				problem("incomplete CPU profile")
				continue
			}
			resolved[entry.Profile.Descriptor] = struct {
				p    profile
				name string
			}{*entry.Profile, entry.Device + " " + entry.CPU}
		case "compile-enter":
			if ready != 1 || entry.Profile == nil || entry.Profile.Descriptor == "" || entry.Profile.Selector == 0 {
				problem("compile without ready/profile")
				continue
			}
			call := &compilation{Sequence: *entry.Sequence, Thread: entry.Thread, Profile: *entry.Profile, Correlation: "thread-order"}
			if named, ok := resolved[call.Profile.Descriptor]; ok && named.p == call.Profile {
				call.ObservedCPU = named.name
			}
			active[entry.Thread] = append(active[entry.Thread], call)
			delete(finished, entry.Thread)
			report.Compilations = append(report.Compilations, call)
		case "compile-leave":
			stack := active[entry.Thread]
			if len(stack) == 0 || entry.Result == nil {
				problem("orphan compile-leave")
				continue
			}
			call := stack[len(stack)-1]
			active[entry.Thread] = stack[:len(stack)-1]
			if entry.CompileSequence != nil {
				call.Correlation = "explicit"
				if *entry.CompileSequence != call.Sequence {
					problem("compile-leave operation mismatch")
				}
			}
			call.Result = entry.Result
			finished[entry.Thread] = call
		case "compiler-file":
			call := finished[entry.Thread]
			if call == nil || call.Result == nil || *call.Result != 1 || call.FileSHA256 != "" {
				problem("file without unique successful compile")
				continue
			}
			if entry.CompileSequence != nil {
				call.Correlation = "explicit"
				if *entry.CompileSequence != call.Sequence {
					problem("compiler file operation mismatch")
				}
			}
			if entry.ByteLength <= 0 || entry.ByteLength > maxImageSize || len(entry.FileHex) != 2*entry.ByteLength {
				problem("invalid compiler file length")
				continue
			}
			image, err := hex.DecodeString(entry.FileHex)
			if err != nil {
				problem("invalid compiler file hex")
				continue
			}
			call.FileSHA256 = digest(image)
		case "capture-error":
			problem("observer reported capture-error")
		case "capture-ended":
			ended++
			if entry.Dropped == nil || *entry.Dropped != 0 {
				problem("missing or nonzero dropped-event count")
			}
		case "instruction", "serialized-instruction", "check-instruction", "program":
			if ready != 1 {
				problem("compiler observation before ready")
			}
		default:
			problem("unknown event kind: " + entry.Kind)
		}
	}
	if err := scanner.Err(); err != nil {
		return report, err
	}
	if modules != 1 || ready != 1 || started != 1 || ended != 1 {
		problem("capture lifecycle incomplete")
	}
	for _, call := range report.Compilations {
		if call.Result == nil {
			problem("compile did not return")
		}
		if call.Result != nil && *call.Result == 1 && call.FileSHA256 == "" {
			problem("successful compile has no captured file")
		}
	}
	report.Complete = len(report.Problems) == 0
	return report, nil
}
