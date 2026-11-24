package commission

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// FileProgramJournal creates an exclusively new directory and immutable backup
// files. Each file/event is synced and closed before success. This is not a
// promise of power-loss durability of directory metadata on every filesystem.
// There is deliberately no reopen/load/resume API. Partial journals are kept.
type FileProgramJournal struct {
	mu       sync.Mutex
	dir      string
	prepared bool
	failed   bool
}

func NewFileProgramJournal(dir string) (*FileProgramJournal, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err = os.Mkdir(absolute, 0700); err != nil {
		return nil, err
	}
	j := &FileProgramJournal{dir: absolute}
	if err = j.create("events.jsonl", nil); err != nil {
		return nil, err
	}
	return j, nil
}

func syncWrite(f *os.File, data []byte) error {
	n, err := f.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = f.Sync()
	}
	return errors.Join(err, f.Close())
}

func (j *FileProgramJournal) create(name string, data []byte) error {
	f, err := os.OpenFile(filepath.Join(j.dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	return syncWrite(f, data)
}

func (j *FileProgramJournal) SavePrepared(bundle PreparedProgramBundle) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.prepared || j.failed {
		return errors.New("journal cannot prepare again")
	}
	// Permanently latch on any partial failure; no append/resume after disk loss.
	j.failed = true
	metadata, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return err
	}
	type savedFile struct {
		name string
		data []byte
	}
	files := []savedFile{
		{"candidate.n1", bundle.Candidate}, {"original-1.um", bundle.OriginalFirst},
		{"original-2.um", bundle.OriginalSecond}, {"observations.json", metadata},
	}
	if len(bundle.SourceArchive) != 0 {
		files = append(files, savedFile{"SRCPRGRM.FBL", bundle.SourceArchive})
	}
	if len(bundle.OriginalCatalog) != 0 {
		files = append(files, savedFile{"original-catalog.bin", bundle.OriginalCatalog})
	}
	if len(bundle.OriginalArchive) != 0 {
		files = append(files, savedFile{"original-SRCPRGRM.FBL", bundle.OriginalArchive})
	}
	type artifact struct {
		Name   string `json:"name"`
		Bytes  int    `json:"bytes"`
		SHA256 string `json:"sha256"`
	}
	manifest := struct {
		Format               string     `json:"format"`
		ProgramOnly          bool       `json:"programOnly"`
		FullControllerBackup bool       `json:"fullControllerBackup"`
		Files                []artifact `json:"files"`
	}{Format: "cj1-program-session-backup-v1", ProgramOnly: true}
	for _, file := range files {
		if err = j.create(file.name, file.data); err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, artifact{file.name, len(file.data), fmt.Sprintf("%x", sha256.Sum256(file.data))})
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err = j.create("prepared.json", raw); err != nil {
		return err
	}
	j.prepared, j.failed = true, false
	return nil
}

func (j *FileProgramJournal) Append(event ProgramSessionEvent) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.failed || (!j.prepared && event.Kind != "stopped") {
		return errors.New("journal unavailable or not prepared")
	}
	raw, err := json.Marshal(event)
	if err != nil {
		j.failed = true
		return err
	}
	f, err := os.OpenFile(filepath.Join(j.dir, "events.jsonl"), os.O_WRONLY|os.O_APPEND, 0600)
	if err == nil {
		err = syncWrite(f, append(raw, '\n'))
	}
	if err != nil {
		j.failed = true
	}
	return err
}
