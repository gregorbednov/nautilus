package commission

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"time"

	"github.com/joyautomation/nautilus/omron/fins"
	"github.com/joyautomation/nautilus/omron/um"
)

func (s *ProgramSession) backupOriginalFiles(ctx context.Context) error {
	catalog, err := s.client.ReadFBSourceCatalog(ctx)
	if err != nil {
		return err
	}
	if catalog.Files > 1 || (catalog.Files == 1 && catalog.ArchiveBytes == 0) {
		return errors.New("CPU catalog contains files outside the scope of the program backup")
	}
	s.originalCatalog = bytes.Clone(catalog.Raw)
	if catalog.ArchiveBytes == 0 {
		return nil
	}
	first, err := s.client.ReadFBSource(ctx, catalog.ArchiveBytes)
	if err != nil {
		return err
	}
	second, err := s.client.ReadFBSource(ctx, catalog.ArchiveBytes)
	if err != nil {
		return err
	}
	if !bytes.Equal(first, second) {
		return errors.New("original SRCPRGRM.FBL backup reads differ")
	}
	s.originalArchive = first
	return nil
}

func (s *ProgramSession) checkOriginalFiles(ctx context.Context, client *fins.Client) error {
	if len(s.originalCatalog) == 0 {
		return nil
	}
	catalog, err := checkSourceArchive(ctx, client, s.originalArchive)
	if err != nil {
		return err
	}
	if !bytes.Equal(catalog.Raw, s.originalCatalog) {
		return errors.New("CPU file catalog differs from the original")
	}
	return nil
}

func checkSourceArchive(ctx context.Context, client *fins.Client, expected []byte) (fins.FBSourceCatalog, error) {
	catalog, err := client.ReadFBSourceCatalog(ctx)
	if err != nil {
		return catalog, err
	}
	if catalog.ArchiveBytes != uint32(len(expected)) {
		return catalog, errors.New("unexpected SRCPRGRM.FBL presence or size")
	}
	if len(expected) == 0 {
		return catalog, nil
	}
	actual, err := client.ReadFBSource(ctx, catalog.ArchiveBytes)
	if err == nil && !bytes.Equal(actual, expected) {
		err = errors.New("SRCPRGRM.FBL bytes differ")
	}
	return catalog, err
}

func (s *ProgramSession) writeSourceArchive(ctx context.Context, phase string, archive []byte) error {
	payload, err := fins.FBSourceWritePayload(archive)
	if err != nil {
		return err
	}
	return s.control(ctx, phase, "2203", hex.EncodeToString(payload), nil, func() error {
		err := s.client.WriteFBSource(ctx, archive)
		var refusal *fins.EndError
		if len(s.originalArchive) == 0 ||
			!errors.As(err, &refusal) || refusal.Command != 0x2203 || refusal.Code != fins.FBSourceChangeRefused {
			return err
		}
		code := refusal.Code
		if journalErr := s.journal.Append(ProgramSessionEvent{Phase: phase, Kind: "control-refused", Command: "2203", EndCode: &code}); journalErr != nil {
			return errors.Join(err, journalErr)
		}
		if guardErr := s.guardSourceArchiveRetry(ctx, phase); guardErr != nil {
			return errors.Join(err, guardErr)
		}
		binding := s.client.Binding()
		if journalErr := s.journal.Append(ProgramSessionEvent{Binding: &binding, Phase: phase, Kind: "control-attempt", Command: "2203", PayloadHex: hex.EncodeToString(payload)}); journalErr != nil {
			return errors.Join(err, journalErr)
		}
		return s.client.WriteFBSource(ctx, archive)
	})
}

func (s *ProgramSession) guardSourceArchiveRetry(ctx context.Context, phase string) error {
	// A CX-Programmer trace shows the same 2203 succeeding after a 2108
	// refusal. Allow one repeat only after the program and file are reconciled.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Second):
	}
	program, err := s.client.ReadProgram(ctx, um.MaxSize)
	if err != nil {
		return err
	}
	matched := bytes.Equal(program, s.original)
	if phase == "candidate-archive" {
		comparison, err := s.candidateAudit.InspectReadback(program)
		if err != nil {
			return err
		}
		matched = comparison.Exact
	}
	if !matched {
		return errors.New("program changed after SRCPRGRM.FBL refusal")
	}
	catalog, err := s.client.ReadFBSourceCatalog(ctx)
	if err != nil {
		return err
	}
	if catalog.Files != 0 {
		if catalog.Files != 1 || (catalog.ArchiveBytes != uint32(len(s.originalArchive)) && catalog.ArchiveBytes != uint32(len(s.archive))) {
			return errors.New("unexpected CPU file catalog after SRCPRGRM.FBL refusal")
		}
		previous, err := s.client.ReadFBSource(ctx, catalog.ArchiveBytes)
		if err != nil {
			return err
		}
		if !bytes.Equal(previous, s.originalArchive) && !bytes.Equal(previous, s.archive) {
			return errors.New("SRCPRGRM.FBL changed after refusal")
		}
	}
	guard, err := s.observe(ctx)
	if err != nil {
		return err
	}
	return s.journal.Append(ProgramSessionEvent{Phase: phase, Kind: "retry-guard", ReadbackData: catalog.Raw, Observation: &guard})
}

func (s *ProgramSession) sendCandidateFiles(ctx context.Context) error {
	const phase = "candidate-archive"
	if len(s.archive) != 0 {
		if err := s.writeSourceArchive(ctx, phase, s.archive); err != nil {
			return err
		}
	}
	catalog, err := checkSourceArchive(ctx, s.client, s.archive)
	if err != nil {
		return err
	}
	program, err := s.client.ReadProgram(ctx, um.MaxSize)
	if err != nil {
		return err
	}
	comparison, err := s.candidateAudit.InspectReadback(program)
	if err != nil {
		return err
	}
	if !comparison.Exact {
		return errors.New("program changed during SRCPRGRM.FBL transfer")
	}
	if err = s.journal.Append(ProgramSessionEvent{Phase: phase, Kind: "catalog-verified", ReadbackData: catalog.Raw}); err != nil {
		return err
	}
	s.archiveTransferred = len(s.archive) != 0
	return nil
}

func (s *ProgramSession) restoreOriginalFiles(ctx context.Context) error {
	if len(s.originalArchive) != 0 {
		if err := s.writeSourceArchive(ctx, "original-archive", s.originalArchive); err != nil {
			return err
		}
	}
	if err := s.checkOriginalFiles(ctx, s.client); err != nil {
		return err
	}
	if len(s.originalArchive) != 0 {
		current, err := s.client.ReadProgram(ctx, um.MaxSize)
		if err != nil {
			return err
		}
		if !bytes.Equal(current, s.original) {
			return errors.New("original program changed during SRCPRGRM.FBL restore")
		}
		if err := s.journal.Append(ProgramSessionEvent{Phase: "original-archive", Kind: "archive-restored"}); err != nil {
			return err
		}
		s.originalArchiveRestored = true
	}
	return nil
}

// VerifyOriginalFiles checks the restored catalog and source on a fresh client.
// The caller should independently check the CPU identity and program.
func (s *ProgramSession) VerifyOriginalFiles(ctx context.Context, fresh *fins.Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if fresh == nil || !s.finished || s.state != "original-matched" {
		return errors.New("original file verification requires a finished original return")
	}
	binding := fresh.Binding()
	if binding.Peer != s.binding.Peer || binding.Route.Destination != s.binding.Route.Destination || binding.Transport != s.binding.Transport {
		return errors.New("original file verification target binding mismatch")
	}
	return s.checkOriginalFiles(ctx, fresh)
}
