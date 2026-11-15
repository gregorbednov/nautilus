package codec_test

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/internal/fbcode"
	"github.com/joyautomation/nautilus/omron/um"
)

func TestInstanceNativeDecodeSourceIdentity(t *testing.T) {
	data, err := os.ReadFile("../um/testdata/cj1h-baseline.n1")
	if err != nil {
		t.Fatal(err)
	}
	image, err := um.ParseN1(data)
	if err != nil {
		t.Fatal(err)
	}
	blocks, err := image.FunctionBlocks()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range loadLibraryProbeResults(t, "instance-decode-manifest.json", 4, "fb-instance-decode-v1") {
		i := 0
		if row.ID == "instance-1" {
			i = 1
		} else if row.ID != "instance-0" {
			t.Fatal("unknown source instance")
		}
		native := blocks.Instances[i]
		if row.SourceSHA256 != sha(data) || row.SourceOffsetWords != int(native.OffsetWords)+len(native.Header) || !reflect.DeepEqual(row.Words, native.Code) || len(row.Decoded) != 17 {
			t.Fatal("native instance input differs from saved N1")
		}
		if row.Decoded[0].Text != []string{"FBIN 0", "FBIN 3"}[i] || row.Decoded[16].Text != "FBIRET" || row.Decoded[13].Text != "FBBCALL &1024" {
			t.Fatal("unexpected instance boundaries")
		}
	}
}

func TestInstanceComponentsAgainstNativeCompileAndControlAPIs(t *testing.T) {
	accepted, rejected, matched := 0, 0, 0
	for _, row := range loadLibraryProbeResults(t, "instance-compile-manifest.json", 110, "fb-instance-compile-v1") {
		if !row.Accepted {
			rejected++
			continue
		}
		accepted++
		var got, want []uint32
		var err error
		switch row.Mnemonic {
		case "FBIN":
			n, _ := strconv.Atoi(row.Operands[0])
			got, _, err = fbcode.InstanceEnvelope(n)
			want = row.Words
		case "FBIRET":
			_, got, err = fbcode.InstanceEnvelope(0)
			want = row.Words
		case "~MOVRW(561)", "~MOVR(560)", "~MOVL(498)":
			bank, _ := strconv.Atoi(strings.TrimPrefix(row.Operands[1], "BR"))
			got, err = fbcode.InstanceBaseMove(bank, row.Operands[0])
			if err != nil {
				if row.Operands[0] != "#0" && row.Operands[0] != "#FFFF" {
					t.Fatal(err)
				}
				continue
			}
			want = row.InstanceStages[0].Words
		case "~BMOV(559)", "~MOV(021)":
			push := strings.Contains(row.Operands[1], "IR62+")
			operand := row.Operands[1]
			if push {
				operand = row.Operands[0]
			}
			offset, _ := strconv.Atoi(strings.Split(operand, ",")[0])
			got, err = fbcode.InstanceTransfer(row.Mnemonic == "~BMOV(559)", push, offset)
			want = row.InstanceStages[0].Words
		case "FBBCALL":
			number, _ := strconv.Atoi(strings.TrimPrefix(row.Operands[0], "&"))
			got, err = fbcode.InstanceBodyCall(number)
			want = row.InstanceStages[1].Words
		case "LD", "RSET", "#SET":
			continue // Final conditional-bit frame handling is checked against saved buffers and the native program pass evidence.
		default:
			t.Fatal("unknown instance component", row.Mnemonic)
		}
		matched++
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatal("instance component differs", row.ID, got, want, err)
		}
	}
	if accepted != 96 || rejected != 14 || matched != 70 {
		t.Fatal("instance component coverage changed", accepted, rejected, matched)
	}
}
