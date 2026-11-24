package um

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestFinalizationWireMappingNativeEvidence(t *testing.T) {
	packed, err := os.ReadFile("testdata/finalization-wire-mapping-evidence.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(packed)
	if hex.EncodeToString(sum[:]) != "8cc5724ae384e2f3a5b92c1c792aadad51cdbf25e78add85c8f9213cd4416ff7" {
		t.Fatal("native mapping evidence changed")
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
			Functions, InstructionCount                                                    int
			VendorCodeExecuted, NetworkAccess, TargetFinalizationQualified, WriteQualified bool
			Checks                                                                         []struct {
				Name   string
				Passed bool
			}
			Globals map[string]string
		}
	}
	if err = json.NewDecoder(z).Decode(&evidence); err != nil {
		t.Fatal(err)
	}
	v := evidence.Verification
	if v.Functions != 8 || v.InstructionCount != 497 || len(v.Checks) != 44 || v.VendorCodeExecuted || v.NetworkAccess || v.TargetFinalizationQualified || v.WriteQualified {
		t.Fatal("incorrect mapping scope")
	}
	for _, c := range v.Checks {
		if !c.Passed {
			t.Fatal("failed native mapping check", c.Name)
		}
	}
	instructions := map[string]string{}
	count := 0
	for _, r := range evidence.Reports {
		if r.SourceSHA256 != "21430e768190361b7dc137ff968e2839ae269d087823f2ca16270076caa9402b" {
			t.Fatal("unknown virtual CPU module")
		}
		for _, f := range r.Functions {
			count++
			if !f.DecompileCompleted || f.Entry != f.RequestedAddress {
				t.Fatal("unverified function")
			}
			for _, i := range f.Instructions {
				instructions[i.Address] = i.Text
			}
		}
	}
	if len(evidence.Reports) != 2 || count != 8 || len(instructions) != 497 || len(evidence.ExpectedInstructions) != 23 {
		t.Fatal("incomplete mapping evidence")
	}
	for address, want := range evidence.ExpectedInstructions {
		if instructions[address] != want {
			t.Fatalf("native mapping at %s: got %q, want %q", address, instructions[address], want)
		}
	}
	// These widths prove only this handler's byte count, not the hardware CPU
	// algorithm or the behavior of other protocol/adaptor layers.
	if instructions["10005873"] != "SHR EAX,0x1" || instructions["1001aa5f"] != "ADD ESI,0x2" || instructions["1001aa65"] != "ADD EAX,0x2" || instructions["1000d259"] != "SHR EAX,0x8" {
		t.Fatal("byte count conflated with step count")
	}
}
