// corpus prepares source-only differential experiments. It never contacts a PLC
// or labels its output as compiled/accepted by CX-Programmer.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joyautomation/nautilus/omron/cxp"
)

type experiment struct {
	Name      string `json:"name"`
	Feature   string `json:"feature"`
	Source    string `json:"source"`
	SHA256    string `json:"cxpSHA256"`
	CXTSHA256 string `json:"cxtSHA256"`
}
type manifest struct {
	SchemaVersion  int          `json:"schemaVersion"`
	Status         string       `json:"status"`
	OriginalSHA256 string       `json:"originalSHA256"`
	Rung           string       `json:"rung"`
	CPUs           []cxp.CPU    `json:"cpus"`
	Warnings       []string     `json:"warnings"`
	Cases          []experiment `json:"cases"`
}

func seeds() []experiment {
	return []experiment{
		{Name: "baseline", Feature: "reference rung with direct W operands", Source: "LD W0.00\nOUT W0.01"},
		{Name: "input-bit", Feature: "input bit 00 -> 15", Source: "LD W0.15\nOUT W0.01"},
		{Name: "input-word", Feature: "input word 0 -> 1", Source: "LD W1.00\nOUT W0.01"},
		{Name: "input-cio", Feature: "input area W -> CIO", Source: "LD CIO0.00\nOUT W0.01"},
		{Name: "input-h", Feature: "input area W -> H", Source: "LD H0.00\nOUT W0.01"},
		{Name: "output-bit", Feature: "output bit 01 -> 02", Source: "LD W0.00\nOUT W0.02"},
		{Name: "inverted-contact", Feature: "LD -> LDNOT", Source: "LDNOT W0.00\nOUT W0.01"},
		{Name: "output-rising", Feature: "OUT -> @OUT (oracle acceptance pending)", Source: "LD W0.00\n@OUT W0.01"},
		{Name: "output-falling", Feature: "OUT -> %OUT (oracle acceptance pending)", Source: "LD W0.00\n%OUT W0.01"},
		{Name: "move-hex", Feature: "replace output with MOV hexadecimal literal", Source: "LD W0.00\nMOV(021) #0001 D0"},
		{Name: "move-decimal", Feature: "MOV literal #0001 -> &1; compare with move-hex", Source: "LD W0.00\nMOV(021) &1 D0"},
		{Name: "timer", Feature: "replace output with TIM", Source: "LD W0.00\nTIM 0 #0010"},
		{Name: "timer-fast", Feature: "TIM -> TIMH(015); compare with timer", Source: "LD W0.00\nTIMH(015) 0 #0010"},
		{Name: "branch", Feature: "parallel condition", Source: "LD W0.00\nOR W0.02\nOUT W0.01"},
	}
}
func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

// prepare replaces exactly one existing textual rung in separate copies of a
// whole project. These are not minimal standalone projects: FBs, tasks, settings
// and every other field remain present. CPU/settings disagreements are retained.
func prepare(source, destination, rung string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	d, err := cxp.Read(data)
	if err != nil {
		return err
	}
	summary := d.Summary()
	if rung == "" {
		return errors.New("an explicit textual -rung path is required")
	}
	if _, err := os.Lstat(destination); err == nil {
		return errors.New("destination already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	parent := filepath.Dir(filepath.Clean(destination))
	temp, err := os.MkdirTemp(parent, ".cj1-corpus-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	if err := cxp.Export(d, data, filepath.Join(temp, "original")); err != nil {
		return err
	}
	m := manifest{SchemaVersion: 1, Status: "source-only; no CX-Programmer acceptance or executable UM captured", OriginalSHA256: digest(data), Rung: rung, CPUs: summary.CPUs, Warnings: summary.Warnings, Cases: seeds()}
	for i := range m.Cases {
		c := &m.Cases[i]
		clone, err := cxp.Read(data)
		if err != nil {
			return err
		}
		if err := clone.SetRung(rung, c.Source); err != nil {
			return err
		}
		text, err := clone.MarshalCXT()
		if err != nil {
			return err
		}
		packed, err := cxp.Pack(text)
		if err != nil {
			return err
		}
		c.SHA256 = digest(packed)
		c.CXTSHA256 = digest(text)
		dir := filepath.Join(temp, c.Name)
		if err := os.Mkdir(dir, 0700); err != nil {
			return err
		}
		for name, content := range map[string][]byte{"project.cxp": packed, "project.cxt": text, "rung.txt": []byte(c.Source + "\n")} {
			if err := os.WriteFile(filepath.Join(dir, name), content, 0600); err != nil {
				return err
			}
		}
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(temp, "manifest.json"), append(out, '\n'), 0600); err != nil {
		return err
	}
	if _, err := os.Lstat(destination); err == nil {
		return errors.New("destination appeared during preparation")
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Rename(temp, destination)
}
func main() {
	source := flag.String("source", "", "original CXP/CXT")
	output := flag.String("o", "", "new corpus directory")
	rung := flag.String("rung", "", "fully qualified existing textual rung path")
	flag.Parse()
	if *source == "" || *output == "" || flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	if err := prepare(*source, *output, *rung); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Prepared source experiments:", *output)
}
