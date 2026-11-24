package cxp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Export writes a lossless research/import bundle into a NEW directory. It
// intentionally does not create nautilus.yaml or claim to produce executable
// LD: graphical FBs and CPU binary encoding still need independent lowering.
// The original file, exact CXT bytes and every parsed field are preserved.
func Export(d *Document, original []byte, destination string) error {
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("destination already exists: %s", destination)
	} else if !os.IsNotExist(err) {
		return err
	}
	parent := filepath.Dir(filepath.Clean(destination))
	dir, err := os.MkdirTemp(parent, ".omron-import-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	summary, err := json.MarshalIndent(d.Summary(), "", "  ")
	if err != nil {
		return err
	}
	tree, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	files := map[string][]byte{"source.cxt": d.Source, "project.json": tree, "report.json": summary, "original." + d.Format: original}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			return err
		}
	}
	// Recheck before rename; never merge into or replace a user's project.
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("destination appeared during import: %s", destination)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(dir, destination); err != nil {
		return err
	}
	return nil
}
