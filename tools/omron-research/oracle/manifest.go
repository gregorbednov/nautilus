package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/um"
)

type artifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}
type sourceRecipe struct {
	Original artifact `json:"original"`
	Rung     string   `json:"rung"`
	Source   string   `json:"source"`
	CXPHash  string   `json:"cxpSHA256"`
	CXTHash  string   `json:"cxtSHA256"`
}
type oracleCase struct {
	ID                  string       `json:"id"`
	CPU                 string       `json:"cpu"`
	ProgramCheckVersion string       `json:"programCheckUnitVersion,omitempty"`
	HardwareUnitVersion *string      `json:"hardwareUnitVersion"`
	CheckLog            *artifact    `json:"checkLog,omitempty"`
	SourceIdentity      string       `json:"sourceIdentity"`
	Source              sourceRecipe `json:"source"`
	Image               artifact     `json:"image"`
	Capture             struct {
		Log             artifact  `json:"log"`
		DecodedSHA256   string    `json:"decodedSHA256"`
		Stderr          artifact  `json:"stderr"`
		Observer        *artifact `json:"observer,omitempty"`
		Binding         string    `json:"fileBinding"`
		CompileSequence uint64    `json:"compileSequence"`
		Profile         profile   `json:"profile"`
	} `json:"capture"`
}
type difference struct {
	Offset int  `json:"offset"`
	Before byte `json:"before"`
	After  byte `json:"after"`
}
type comparison struct {
	Left        string       `json:"left"`
	Right       string       `json:"right"`
	Differences []difference `json:"differences"`
}
type manifest struct {
	Schema      int          `json:"schemaVersion"`
	Cases       []oracleCase `json:"cases"`
	Comparisons []comparison `json:"comparisons"`
}
type verifiedCase struct {
	ID                               string      `json:"id"`
	CPU                              string      `json:"declaredCPU"`
	CPUIdentitySource                string      `json:"cpuIdentitySource"`
	SourceRecipeVerified             bool        `json:"sourceRecipeVerified"`
	SourceIdentityInCompilerVerified bool        `json:"sourceIdentityInCompilerVerified"`
	CaptureComplete                  bool        `json:"captureComplete"`
	BinaryBindingVerified            bool        `json:"binaryBindingVerified"`
	Image                            um.N1Report `json:"image"`
}
type manifestReport struct {
	Schema            int            `json:"schemaVersion"`
	Cases             []verifiedCase `json:"cases"`
	Comparisons       int            `json:"comparisonsVerified"`
	HardwareQualified bool           `json:"hardwareQualified"`
}

var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

var checkSummaryPattern = regexp.MustCompile(`(?m)^[^\r\n]+ - ([0-9]+) errors, ([0-9]+) warnings\.\r?$`)

func validateSourceCheck(data []byte, cpu, version string) error {
	text := string(data)
	summaries := checkSummaryPattern.FindAllStringSubmatch(text, -1)
	if strings.Count(text, "PLC Model '") != 1 || !strings.Contains(text, "PLC Model '"+cpu+"'") || len(summaries) != 1 || summaries[0][1] != "0" || version == "" || !strings.Contains(text, "The programs have been checked with the program check option set to Unit Ver."+version+".") {
		return fmt.Errorf("source check model, result or version mismatch")
	}
	return nil
}

func localPath(root, name string) (string, error) {
	if !fs.ValidPath(name) || strings.ContainsAny(name, `\:`) {
		return "", fmt.Errorf("nonportable artifact path: %q", name)
	}
	full, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, full)
	if err != nil || !filepath.IsLocal(relative) {
		return "", fmt.Errorf("artifact escapes corpus root: %q", name)
	}
	return full, nil
}
func loadArtifact(root string, entry artifact, limit int) ([]byte, error) {
	if !hashPattern.MatchString(entry.SHA256) || entry.Bytes < 0 || entry.Bytes > limit {
		return nil, fmt.Errorf("invalid artifact bounds/hash: %s", entry.Path)
	}
	path, err := localPath(root, entry.Path)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := readBounded(file, limit)
	if err != nil {
		return nil, err
	}
	if len(data) != entry.Bytes || digest(data) != entry.SHA256 {
		return nil, fmt.Errorf("artifact size/hash mismatch: %s", entry.Path)
	}
	return data, nil
}

func verifyManifestReader(root string, reader io.Reader) (manifestReport, error) {
	report := manifestReport{Schema: 1, Cases: []verifiedCase{}}
	root, err := filepath.Abs(root)
	if err != nil {
		return report, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return report, err
	}
	data, err := readBounded(reader, 1<<20)
	if err != nil {
		return report, err
	}
	var corpus manifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		return report, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return report, fmt.Errorf("trailing manifest JSON")
	}
	if corpus.Schema != 1 || len(corpus.Cases) == 0 || len(corpus.Cases) > 64 {
		return report, fmt.Errorf("invalid manifest schema/case count")
	}
	images := map[string][]byte{}
	for _, entry := range corpus.Cases {
		if entry.ID == "" || images[entry.ID] != nil {
			return report, fmt.Errorf("empty/duplicate case id: %q", entry.ID)
		}
		if entry.CPU == "" || entry.SourceIdentity != "operator-reported" {
			return report, fmt.Errorf("unsupported source identity claim in %s", entry.ID)
		}
		if entry.HardwareUnitVersion != nil {
			return report, fmt.Errorf("hardware version has no measurement evidence: %s", entry.ID)
		}
		if entry.CheckLog != nil {
			check, err := loadArtifact(root, *entry.CheckLog, 1<<20)
			if err != nil {
				return report, err
			}
			if validateSourceCheck(check, entry.CPU, entry.ProgramCheckVersion) != nil {
				return report, fmt.Errorf("source check log mismatch: %s", entry.ID)
			}
		} else if entry.ProgramCheckVersion != "" {
			return report, fmt.Errorf("program check version without log: %s", entry.ID)
		}
		original, err := loadArtifact(root, entry.Source.Original, cxp.MaxSize)
		if err != nil {
			return report, err
		}
		document, err := cxp.Read(original)
		if err != nil {
			return report, err
		}
		if err := document.SetRung(entry.Source.Rung, entry.Source.Source); err != nil {
			return report, err
		}
		cxt, err := document.MarshalCXT()
		if err != nil {
			return report, err
		}
		cxpBytes, err := cxp.Pack(cxt)
		if err != nil {
			return report, err
		}
		if digest(cxt) != entry.Source.CXTHash || digest(cxpBytes) != entry.Source.CXPHash {
			return report, fmt.Errorf("source recipe does not reproduce %s", entry.ID)
		}
		image, err := loadArtifact(root, entry.Image, um.MaxSize)
		if err != nil {
			return report, err
		}
		parsed, err := um.ParseN1(image)
		if err != nil {
			return report, fmt.Errorf("case %s: %w", entry.ID, err)
		}
		compressed, err := loadArtifact(root, entry.Capture.Log, maxLogSize)
		if err != nil {
			return report, err
		}
		zip, err := gzip.NewReader(bytes.NewReader(compressed))
		if err != nil {
			return report, err
		}
		transcript, err := readBounded(zip, maxLogSize)
		zip.Close()
		if err != nil {
			return report, err
		}
		if digest(transcript) != entry.Capture.DecodedSHA256 {
			return report, fmt.Errorf("decoded transcript hash mismatch: %s", entry.ID)
		}
		stderr, err := loadArtifact(root, entry.Capture.Stderr, 1<<20)
		if err != nil {
			return report, err
		}
		if entry.Capture.Observer != nil {
			if _, err := loadArtifact(root, *entry.Capture.Observer, 1<<20); err != nil {
				return report, err
			}
		}
		audit, err := auditCapture(bytes.NewReader(transcript), string(stderr))
		if err != nil {
			return report, err
		}
		var selected *compilation
		for _, call := range audit.Compilations {
			if call.Sequence == entry.Capture.CompileSequence {
				selected = call
			}
		}
		if selected == nil || selected.Result == nil || *selected.Result != 1 {
			return report, fmt.Errorf("successful compile missing: %s", entry.ID)
		}
		want := entry.Capture.Profile
		got := selected.Profile
		if want.Selector != got.Selector || want.Low != got.Low || want.High != got.High {
			return report, fmt.Errorf("compile profile mismatch: %s", entry.ID)
		}
		if selected.ObservedCPU != "" && selected.ObservedCPU != entry.CPU {
			return report, fmt.Errorf("observed CPU mismatch: %s", entry.ID)
		}
		verified := verifiedCase{ID: entry.ID, CPU: entry.CPU, CPUIdentitySource: "operator+compile-profile", SourceRecipeVerified: true, CaptureComplete: audit.Complete, Image: parsed.Summary()}
		if selected.ObservedCPU != "" {
			verified.CPUIdentitySource = "runtime-initialization"
		}
		switch entry.Capture.Binding {
		case "embedded-file":
			if !audit.Complete || selected.FileSHA256 != entry.Image.SHA256 {
				return report, fmt.Errorf("incomplete or mismatched embedded capture %s: %v", entry.ID, audit.Problems)
			}
			verified.BinaryBindingVerified = true
		case "operator-copy":
			if selected.FileSHA256 != "" || len(audit.Problems) != 1 || audit.Problems[0] != "successful compile has no captured file" {
				return report, fmt.Errorf("historical capture has unexpected defects: %s", entry.ID)
			}
		default:
			return report, fmt.Errorf("unsupported binary binding: %s", entry.ID)
		}
		report.Cases = append(report.Cases, verified)
		images[entry.ID] = image
	}
	for _, pair := range corpus.Comparisons {
		left, right := images[pair.Left], images[pair.Right]
		if left == nil || right == nil || len(left) != len(right) {
			return report, fmt.Errorf("invalid comparison %s/%s", pair.Left, pair.Right)
		}
		actual := []difference{}
		for index := range left {
			if left[index] != right[index] {
				actual = append(actual, difference{index, left[index], right[index]})
			}
		}
		if len(actual) != len(pair.Differences) {
			return report, fmt.Errorf("difference count mismatch: %s/%s", pair.Left, pair.Right)
		}
		for index := range actual {
			if actual[index] != pair.Differences[index] {
				return report, fmt.Errorf("byte difference mismatch: %s/%s", pair.Left, pair.Right)
			}
		}
		report.Comparisons++
	}
	return report, nil
}
