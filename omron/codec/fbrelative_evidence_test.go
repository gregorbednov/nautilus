package codec_test

import (
	"encoding/json"
	"encoding/xml"
	"reflect"
	"testing"
)

func TestFBRelativeNativeAPIAndSignedOffsetEvidence(t *testing.T) {
	for _, spec := range []struct {
		artifact oracleArtifact
		entries  []string
	}{
		{oracleArtifact{Path: "ghidra-fb-decode-01.json.gz", SHA256: "ed7190e0367d39c9a8666839807d6c5f1cc1d8ddac43489dbade98b979c98d4c", Bytes: 5446, Encoding: "gzip", Decoded: "c8353dd73d0118c7c3ec9cb643943d0c59c1adebd988b40401663882003c3ce2"}, []string{"121e8b11", "121e8bd0", "1216f154", "121d6216"}},
		{oracleArtifact{Path: "ghidra-program-decode-02.json.gz", SHA256: "29ca76fbbb8e12a23439dc72879e11dab2493d946b2a537bf97cb14e8dda7559", Bytes: 18006, Encoding: "gzip", Decoded: "98d10a83377c6e1d4205b280610ca1cb064b019947d2b4db8a8875ed87820f7c"}, []string{"121d5677", "121d5c77", "1202284a"}},
		{oracleArtifact{Path: "ghidra-program-text-03.json.gz", SHA256: "07e4be0a1a8568c94b6fcb9c1a53fc7a5d80d808dedb041c2486d0de930ad4b1", Bytes: 4389, Encoding: "gzip", Decoded: "f403b88d84f40be44296482d173a9452ca75873970f5cbe609169930581d4758"}, []string{"121d62a8", "1216c2ae", "121d544a", "12013d74"}},
		{oracleArtifact{Path: "ghidra-br-parser-04.json.gz", SHA256: "016000ca384950698145406b4d1eaffa3c9879fc56d0a93b637e4517a32b38c6", Bytes: 8432, Encoding: "gzip", Decoded: "99503b26bc69fad24ae0b4b90ecae2be494e209922f34c024b94884a6c3030ad"}, []string{"1203140f"}},
		{oracleArtifact{Path: "ghidra-fb-mode-05.json.gz", SHA256: "0d6a5dc9a3ffcfd1cd1b62423fb8c002858dbffaaa4ad2cbcf52756f34c913e2", Bytes: 973, Encoding: "gzip", Decoded: "6ddbbb322ee038aedecd882f8b8410e17cca3696a43c11e0478adfcafbbd102b"}, []string{"121cf742"}},
		{oracleArtifact{Path: "ghidra-fb-source-path-06.json.gz", SHA256: "ba394fb58829060b1f72135d4179c96abe678e56c868c3b9892638233c832147", Bytes: 3236, Encoding: "gzip", Decoded: "c1dba035367ac378e0463f2752909f2bdd451233701f20e5c1d4974ece12d11b"}, []string{"121e3bde", "121b0e14", "121cf79a"}},
		{oracleArtifact{Path: "ghidra-fb-initialise-07.json.gz", SHA256: "e56a7fa0f7f226592ba9fbb584287aeef18a93ab8ea18f556ab71958b00fe148", Bytes: 7992, Encoding: "gzip", Decoded: "3657b6c1c45dc341962f9757c1f9da23b202a099719c1ad5d01a16b3c79e0bcd"}, []string{"1216a9e0", "121ba80e", "12012c60", "12012580"}},
		{oracleArtifact{Path: "ghidra-instruction-compile.json.gz", SHA256: "a2de4622dabb5c723cf250da6b7ddbe67eda5a319a7830d16d5a58bd01e6efd7", Bytes: 53492, Encoding: "gzip", Decoded: "6912ef375ce96a5c225e80525a158cb7b312d3c45e73064a5cc1ea6bccc55f92"}, []string{"1216b407", "1216b432", "121e3eef"}},
		{oracleArtifact{Path: "ghidra-definition-properties-09.json.gz", SHA256: "40cfad6df77c6adee83ca3318c1bdaf45f5f0d22589c3594e366d3138a470b56", Bytes: 1478, Encoding: "gzip", Decoded: "56249016c5f15ae0b7e901066df679b24bef0c3dc7fc2c5566d9daa7efc8bb8f"}, []string{"12012e60", "1221eb8f", "1221b323", "1221b34b", "12012d80"}},
		{oracleArtifact{Path: "ghidra-instance-envelope-10.json.gz", SHA256: "2f4b4cb9ff7735b7673f90a8f2571d4ee1afa52a3d9c6c38f09ba4cc7f5fe37b", Bytes: 2819, Encoding: "gzip", Decoded: "1ca61243b3099b827844ceb83f207c1024f75072c97bf2049459299768270e8a"}, []string{"121e3d2f", "121d6276", "1221eb8f"}},
		{oracleArtifact{Path: "ghidra-section-setter-12.json.gz", SHA256: "50fe286cd583c160be498414129301714be5da82a9d20ab916a473133e6ff219", Bytes: 4745, Encoding: "gzip", Decoded: "278c755b2e1d00f7831d95a4aec8a06298a03db799a53ea5c17197a93a043d2e"}, []string{"121cf6e3", "121b96e5"}},
		{oracleArtifact{Path: "ghidra-task-properties-13.json.gz", SHA256: "6ac2b1a06a5ef27429cc1d3f6a5e5f7933af6e7c9b290414f79a78c3f5870a46", Bytes: 2329, Encoding: "gzip", Decoded: "9a900a1fc0c9c2e756bc3c38a4802e6e1b4396c7044a2e4d062fbdc7bd13ddbc"}, []string{"12215f70", "1221602c", "12012ce0", "12012ca0", "121ca1ee"}},
		{oracleArtifact{Path: "ghidra-header-state-14.json.gz", SHA256: "c93fa0350783f983221dcb9336ccdad0850302391725d78e2f5775cd89673387", Bytes: 6477, Encoding: "gzip", Decoded: "854d73a34058aac231028d3567f6defe1e329305c121c56974c889e32c2f7e1a"}, []string{"12215fb8", "12012cc0", "121cbaed", "121cbecf", "121ce752", "121ceb6d"}},
	} {
		var proof struct {
			Engine, EngineVersion, SourceSHA256 string
			Functions                           []struct {
				Entry, Name, Pseudocode string
				DecompileCompleted      bool
				Instructions            []struct{ Address, Text string }
			}
		}
		if err := json.Unmarshal(loadOracleArtifact(t, spec.artifact), &proof); err != nil {
			t.Fatal(err)
		}
		if proof.Engine != "ghidra" || proof.EngineVersion != "12.1.4" || proof.SourceSHA256 != "a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141" {
			t.Fatal("static provenance changed")
		}
		var entries []string
		for _, function := range proof.Functions {
			if !function.DecompileCompleted || function.Name == "" || function.Pseudocode == "" || len(function.Instructions) == 0 {
				t.Fatal("incomplete API evidence")
			}
			entries = append(entries, function.Entry)
		}
		if !reflect.DeepEqual(entries, spec.entries) {
			t.Fatal("unexpected function exports", entries)
		}
	}
}

func TestNativeCompileMFCOrdinals(t *testing.T) {
	artifact := oracleArtifact{Path: "mfc42-ordinals.exports.gz", SHA256: "e6f36ab6fb33523ce764a396efe50b4766c2ce0e9da13838b2243c8127ec50e5", Bytes: 80884, Encoding: "gzip", Decoded: "0f740a5da18ff483c27a1b12c5d0c9b7cce23ee35c3046493ceb4f8a025f5938"}
	var library struct {
		Name    string `xml:"NAME,attr"`
		Exports []struct {
			Ordinal int    `xml:"ORDINAL,attr"`
			Name    string `xml:"NAME,attr"`
		} `xml:"EXPORT"`
	}
	if err := xml.Unmarshal(loadOracleArtifact(t, artifact), &library); err != nil {
		t.Fatal(err)
	}
	if library.Name != "mfc42.dll" {
		t.Fatal("wrong ordinal library")
	}
	want := map[int]string{541: "??0CStringArray@@QAE@XZ", 801: "??1CStringArray@@UAE@XZ", 5861: "?SetAtGrow@CStringArray@@QAEXHPBD@Z"}
	for _, entry := range library.Exports {
		if name, ok := want[entry.Ordinal]; ok {
			if entry.Name != name {
				t.Fatal("ordinal signature mismatch", entry)
			}
			delete(want, entry.Ordinal)
		}
	}
	if len(want) != 0 {
		t.Fatal("missing reviewed ordinal", want)
	}
}

func TestFBModeAloneDoesNotQualifyRelativeEncoding(t *testing.T) {
	baseline := loadLibraryProbeResults(t, "br-rendered-manifest.json", 72, "fb-body-operands-v1")
	configured := loadLibraryProbeResults(t, "br-context-manifest.json", 72, "fb-body-context-v1")
	for i, row := range configured {
		if row.Accepted || !reflect.DeepEqual(row.OperandResults, baseline[i].OperandResults) || len(row.Words) != 0 {
			t.Fatal("FB mode changed standalone operand result", row.ID)
		}
	}
}

func TestRenderedBRTextDoesNotQualifyNativeEncoding(t *testing.T) {
	for _, spec := range []struct {
		name  string
		count int
	}{{"br-rendered-manifest.json", 72}, {"br-typed-manifest.json", 30}} {
		for _, row := range loadLibraryProbeResults(t, spec.name, spec.count, "fb-body-operands-v1") {
			if row.Accepted || len(row.Words) != 0 || row.EncodeResult != nil || len(row.OperandResults) == 0 {
				t.Fatal("unexpected BR encoding success", row.ID)
			}
			failed := false
			for _, result := range row.OperandResults {
				failed = failed || result != 0
			}
			if !failed {
				t.Fatal("BR form rejected without a failed operand", row.ID)
			}
		}
	}
}
