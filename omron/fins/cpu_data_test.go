package fins

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestCPUDataRetainedPrefixObservations(t *testing.T) {
	data, err := os.ReadFile("testdata/cpu-unit-data-prefix-observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		SchemaVersion int
		Cases         []struct {
			Name, Model, InternalSystemVersion, RawPrefixHex, StatusHex string
			ProgramAreaKWords                                           uint16
			ReportedProgramAreaBytes                                    uint32
			PrefixOnly                                                  bool
			OriginalPayloadBytes                                        int
		}
	}
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.SchemaVersion != 1 || len(corpus.Cases) != 2 {
		t.Fatal("missing retained prefixes")
	}
	for _, row := range corpus.Cases {
		t.Run(row.Name, func(t *testing.T) {
			raw, err := hex.DecodeString(row.RawPrefixHex)
			if err != nil {
				t.Fatal(err)
			}
			saved := bytes.Clone(raw)
			cpu, err := DecodeCPUUnitData(raw)
			if err != nil {
				t.Fatal(err)
			}
			if !row.PrefixOnly || row.OriginalPayloadBytes != 158 || len(raw) != 92 || cpu.Model != row.Model || cpu.InternalSystemVersion != row.InternalSystemVersion || !strings.ContainsRune(cpu.InternalSystemVersion, 0) || cpu.ProgramAreaKWords != row.ProgramAreaKWords || cpu.ReportedProgramAreaBytes != row.ReportedProgramAreaBytes || cpu.IOMKBytes != 23 || cpu.DMWords != 32768 || cpu.TimerCounterUnits1024 != 8 || cpu.MemoryCardType != 0 || cpu.MemoryCardKBytes != 0 || cpu.UninterpretedTailBytes != 0 {
				t.Fatal("CPU field offsets/units changed", cpu)
			}
			if !bytes.Equal(raw, saved) {
				t.Fatal("decoder changed retained bytes")
			}
			identity, err := DecodeIdentityData(raw)
			if err != nil {
				t.Fatal(err)
			}
			identity.Raw[0] ^= 1
			if !bytes.Equal(raw, saved) {
				t.Fatal("identity aliases caller")
			}
			statusRaw, err := hex.DecodeString(row.StatusHex)
			if err != nil {
				t.Fatal(err)
			}
			status, err := DecodeStatusData(statusRaw)
			if err != nil || status.Mode != 4 || status.Fatal != 0 || status.NonFatal != 0 {
				t.Fatal("retained status changed", status, err)
			}
			status.Raw[1] = 0
			if statusRaw[1] != 4 {
				t.Fatal("status aliases caller")
			}
		})
	}
}

func TestCPUDataBaseAndOpaqueExtension(t *testing.T) {
	raw := make([]byte, 92)
	copy(raw, "CJ1M_CPU21")
	copy(raw[20:], "01.00\x00internal")
	raw[80] = 0
	raw[81] = 10
	raw[82] = 23
	raw[83] = 0x80
	raw[85] = 8
	raw[89] = 4
	raw[90] = 1
	raw[91] = 0
	base, err := DecodeCPUUnitData(raw)
	if err != nil {
		t.Fatal(err)
	}
	extended, err := DecodeCPUUnitData(append(bytes.Clone(raw), 0x12, 0x34, 0x56))
	if err != nil {
		t.Fatal(err)
	}
	if base.ReportedProgramAreaBytes != 20480 || base.MemoryCardKBytes != 256 || extended.UninterpretedTailBytes != 3 || extended.PayloadSHA256 == base.PayloadSHA256 || extended.ProgramAreaKWords != base.ProgramAreaKWords {
		t.Fatal("extension interpreted or units changed")
	}
	if _, err := DecodeCPUUnitData(raw[:91]); err == nil {
		t.Fatal("partial area block accepted")
	}
	// Preserve the existing basic client's40-byte identity compatibility.
	if id, err := DecodeIdentityData(raw[:40]); err != nil || len(id.Raw) != 40 {
		t.Fatal("basic identity compatibility lost", err)
	}
	for _, data := range [][]byte{nil, raw[:39]} {
		if _, err := DecodeIdentityData(data); err == nil {
			t.Fatal("short identity accepted")
		}
	}
	if _, err := DecodeStatusData(make([]byte, 25)); err == nil {
		t.Fatal("short status accepted")
	}
	bad := make([]byte, 26)
	bad[1] = 1
	if _, err := DecodeStatusData(bad); err == nil {
		t.Fatal("unknown mode accepted")
	}
}
