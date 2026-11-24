package transfer_test

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/joyautomation/nautilus/omron/transfer"
	"github.com/joyautomation/nautilus/omron/um"
)

// Static byte-path evidence is replayed without loading any vendor library.
func TestProgramFileBytePathNativeEvidence(t *testing.T) {
	packed, err := os.ReadFile("testdata/file-byte-path-evidence.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(packed)
	if hex.EncodeToString(sum[:]) != "6ca07a82241c59bf0519fe8f1f2469d103264db2421154cb96fcfbc091e97662" {
		t.Fatal("file byte-path evidence changed")
	}
	z, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var evidence struct {
		Reports []struct {
			SourceSHA256 string
			Functions    []struct {
				Entry, RequestedAddress string
				DecompileCompleted      bool
				Instructions            []struct{ Address, Text string }
			}
		}
		ExpectedInstructions map[string]string
		Verification         struct {
			NetworkAccess, VendorCodeExecuted, WireImageMappingQualified, WriteQualified bool
			Functions, InstructionCount                                                  int
			Checks                                                                       []struct {
				Name   string
				Passed bool
			}
			Vtables []struct{ Address, Target string }
			Imports []struct{ Address, Resolved string }
		}
	}
	if err := json.NewDecoder(z).Decode(&evidence); err != nil {
		t.Fatal(err)
	}
	v := evidence.Verification
	if v.NetworkAccess || v.VendorCodeExecuted || v.WireImageMappingQualified || v.WriteQualified || len(v.Checks) != 125 || v.Functions != 21 || v.InstructionCount != 7156 {
		t.Fatal("incorrect evidence scope")
	}
	for _, check := range v.Checks {
		if !check.Passed {
			t.Fatal("failed static check", check.Name)
		}
	}
	instructions := map[string]string{}
	count := 0
	for _, report := range evidence.Reports {
		switch report.SourceSHA256 {
		case "e91f15b8613e56c8f326bc83ce07f5a24b177f5f70a4a445e4eadfa34b0a9eb4", "2abef36d39a307a88ecee7283e841688a1004e909197558ad3368fe4d528b65b":
		default:
			t.Fatal("unexpected native source")
		}
		for _, f := range report.Functions {
			count++
			if !f.DecompileCompleted || f.Entry != f.RequestedAddress {
				t.Fatal("unverified function boundary")
			}
			for _, i := range f.Instructions {
				instructions[i.Address] = i.Text
			}
		}
	}
	if len(evidence.Reports) != 6 || count != 21 || len(instructions) != 7156 || len(evidence.ExpectedInstructions) < 55 {
		t.Fatal("incomplete byte-path evidence")
	}
	for address, want := range evidence.ExpectedInstructions {
		if instructions[address] != want {
			t.Fatalf("byte-path instruction %s: got %q, want %q", address, instructions[address], want)
		}
	}
	vt := map[string]string{}
	for _, item := range v.Vtables {
		vt[item.Address] = item.Target
	}
	if len(vt) != 3 || vt["0x63045700"] != "0x63019460" || vt["0x63045714"] != "0x63019780" || vt["0x630456a0"] != "0x63018c20" {
		t.Fatal("capacity adjuster or Init binding changed")
	}
	imports := map[string]string{}
	for _, item := range v.Imports {
		imports[item.Address] = item.Resolved
	}
	if imports["0x1285ace8"] != "CX-Pdata.dll:?HasAccessRights@PLC_KBase@@QAEHXZ" || imports["0x1285af82"] != "??0CString@@QAE@ABV0@@Z" || imports["0x1285af34"] != "??4CString@@QAEABV0@PBD@Z" {
		t.Fatal("filename provenance imports changed")
	}
}

func TestProgramPlanRequiresExplicitObjectUnwrapping(t *testing.T) {
	for _, name := range []string{"cj1h-baseline.n1", "cj1m-input-bit.n1"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile("../um/testdata/" + name)
			if err != nil {
				t.Fatal(err)
			}
			object, err := um.WrapPlainN1Object(raw)
			if err != nil {
				t.Fatal(err)
			}
			if plan, err := transfer.PlanProgramWrite(object, 996); plan != nil || !errors.Is(err, um.ErrN1Layout) {
				t.Fatal("OBJ accepted as raw N1; service path does not remove its prefix")
			}
			image, err := um.ParsePlainN1Object(object)
			if err != nil {
				t.Fatal(err)
			}
			direct, err := transfer.PlanProgramWrite(raw, 996)
			if err != nil {
				t.Fatal(err)
			}
			explicit, err := transfer.PlanProgramWrite(image.Original(), 996)
			if err != nil {
				t.Fatal(err)
			}
			if direct.ImageSHA256 != explicit.ImageSHA256 || direct.ImageBytes != len(raw) || !slices.Equal(direct.Blocks, explicit.Blocks) {
				t.Fatal("explicit unwrapping changed raw image blocks")
			}
			if direct.NetworkAccess || direct.ImageModified || direct.WireImageMappingQualified || direct.WriteQualified || explicit.WriteQualified {
				t.Fatal("format evidence incorrectly enabled deployment")
			}
		})
	}
}
