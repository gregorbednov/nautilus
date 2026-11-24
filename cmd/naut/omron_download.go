package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/commission"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/fins"
	"github.com/joyautomation/nautilus/omron/um"
)

func omronDownload(args []string) (err error) {
	flags := flag.NewFlagSet("omron download", flag.ContinueOnError)
	model := flags.String("model", "", "explicit CPU model")
	resource := flags.String("resource", "Resource[0]", "fully qualified resource path")
	timer := flags.String("timer-mode", "", "explicit timer mode: bcd or binary")
	encoding := flags.String("encoding", "auto", "CXT text encoding")
	settingsPath := flags.String("settings", "", "image settings JSON")
	project := flags.Bool("project", false, "use a CJ1 project directory instead of standalone source and settings")
	endpoint := flags.String("endpoint", "", "direct FINS/TCP IP:PORT")
	node := flags.Int("node", 0, "destination FINS node; 0 uses the handshake")
	timeout := flags.Duration("timeout", 4*time.Second, "per-exchange timeout")
	targetPath := flags.String("target", "", "retained CPU identity/status JSON from omron status")
	journalPath := flags.String("journal", "", "new directory for backup and journal")
	residentPath := flags.String("resident-reference", "", "retained new4 resident UM for the selected model")
	originalPath := flags.String("expected-original", "", "exact original UM expected on the PLC")
	execute := flags.Bool("execute", false, "authorize the program write")
	run := flags.Bool("run", false, "explicitly request RUN after verification")
	returnOriginal := flags.Bool("return-original", false, "return the pinned original and leave PROGRAM")
	taskCount := flags.Uint("check-task-flags", 0, "verify this many consecutive active flags starting at TK0")
	if err = flags.Parse(args); err != nil {
		return err
	}
	addr, addrErr := netip.ParseAddrPort(*endpoint)
	if flags.NArg() != 1 || *targetPath == "" || *journalPath == "" || addrErr != nil || addr.Port() == 0 {
		return errors.New("download requires SOURCE or -project DIRECTORY, -target, -endpoint IP:PORT and -journal NEW_DIR")
	}
	if !*execute {
		return errors.New("download requires explicit -execute")
	}
	if *node < 0 || *node > 254 || *timeout <= 0 || *taskCount > 32 || (*taskCount != 0 && !*run) || (*returnOriginal && *originalPath == "") {
		return errors.New("invalid node, timeout, task flags or original return options")
	}
	sourcePath := flags.Arg(0)
	var checkVersion string
	if *project {
		if *model != "" || *timer != "" || *settingsPath != "" || *resource != "Resource[0]" || *encoding != "auto" {
			return errors.New("download -project uses the project's own compiler settings")
		}
		p, err := readOmronProject(sourcePath)
		if err != nil {
			return err
		}
		if p.SourceFormat != "" {
			return errors.New("download -project requires a CXT ladder project")
		}
		*model, *timer, *resource, checkVersion = p.Model, p.TimerMode, p.Resource, p.CheckVersion
		*settingsPath = filepath.Join(sourcePath, "settings.json")
		sourcePath = filepath.Join(sourcePath, "program.cxt")
	} else if *model == "" || *settingsPath == "" {
		return errors.New("standalone download requires -model, -timer-mode and -settings")
	}
	settingsData, err := readOmronFile(*settingsPath)
	if err != nil {
		return err
	}
	settings, err := parseOmronImageSettings(settingsData)
	if err != nil {
		return err
	}
	source, err := readOmronFile(sourcePath)
	if err != nil {
		return err
	}
	if checkVersion == "3.0" {
		doc, err := cxp.ReadEncoding(source, *encoding)
		if err != nil {
			return err
		}
		if err := checkOmronProjectSource(doc, *resource, checkVersion); err != nil {
			return err
		}
	}
	profile := codec.Profile{Model: *model, TimerCounterMode: codec.TimerCounterMode(*timer)}
	proof, image, err := compiler.BuildCXPImageForDownload(profile, source, *encoding, *resource, settings)
	if err != nil {
		return err
	}
	target, err := readOmronCommissionSnapshot(*targetPath)
	if err != nil {
		return err
	}
	cfg := commission.ProgramSessionConfig{
		Model: *model, ExpectedPeer: addr.String(), ExpectedCPUData: target.CPU.Raw,
		Candidate: image, Build: proof, BlockBytes: 996, RequireInitialProgram: true,
		PreserveFBSource: *model == codec.CJ1MCPU21,
	}
	if *residentPath != "" {
		resident, err := omronAuditReadFile(*residentPath, um.MaxSize)
		if err != nil {
			return err
		}
		if *model == codec.CJ1HCPU65 {
			cfg.CJ1HNew4Readback = resident
		} else {
			cfg.CJ1MNew4Readback = resident
		}
	}
	if *originalPath != "" {
		cfg.ExpectedOriginal, err = omronAuditReadFile(*originalPath, um.MaxSize)
		if err != nil {
			return err
		}
		if len(cfg.ExpectedOriginal) == 0 {
			return errors.New("expected original UM is empty")
		}
	}
	journal, err := commission.NewFileProgramJournal(*journalPath)
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	route := fins.Route{Destination: fins.Address{Node: byte(*node)}}
	dial := func(ctx context.Context) (*fins.Client, error) {
		return fins.DialTCP(ctx, cfg.ExpectedPeer, route, *timeout)
	}
	client, err := dial(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	session, err := commission.NewControlledProgramSession(client, cfg, journal)
	if err != nil {
		return err
	}
	defer session.Close()
	report := struct {
		Session          commission.ProgramSessionSnapshot `json:"session"`
		TaskFlags        string                            `json:"taskFlags,omitempty"`
		OriginalVerified bool                              `json:"originalVerified,omitempty"`
	}{}
	defer func() {
		report.Session = session.Snapshot()
		err = errors.Join(err, json.NewEncoder(os.Stdout).Encode(report))
	}()
	if err = session.Prepare(ctx); err != nil {
		return err
	}
	if err = session.EnterProgram(ctx); err != nil {
		return err
	}
	err = session.WriteCandidate(ctx)
	if err == nil && *run {
		err = session.RequestExecution(ctx, fins.CPUModeRun)
	}
	if err == nil && *taskCount != 0 {
		var taskFlags []byte
		taskFlags, err = session.ReadExecutionTasks(ctx, uint16(*taskCount))
		report.TaskFlags = hex.EncodeToString(taskFlags)
		if err == nil && !bytes.Equal(taskFlags, bytes.Repeat([]byte{1}, int(*taskCount))) {
			err = fmt.Errorf("inactive task in TK0..TK%d: %s", *taskCount-1, report.TaskFlags)
		}
	}
	if *returnOriginal && session.Snapshot().Candidate.AttemptedBlocks != 0 {
		// The requested return must have its own time budget even if the
		// transfer or RUN observation was interrupted.
		recovery, cancelRecovery := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelRecovery()
		restoreErr := restoreOmronDownload(recovery, session, dial, cfg)
		report.OriginalVerified = restoreErr == nil
		return errors.Join(err, restoreErr)
	}
	if session.Snapshot().State != "candidate-matched" {
		return err
	}
	return errors.Join(err, session.FinishProgram(ctx))
}

func restoreOmronDownload(ctx context.Context, session *commission.ProgramSession, dial func(context.Context) (*fins.Client, error), cfg commission.ProgramSessionConfig) error {
	if session.Snapshot().State == "candidate-matched" {
		if err := session.RestoreOriginalInSession(ctx); err != nil {
			return fmt.Errorf("restore original: %w", err)
		}
	} else {
		fresh, err := dial(ctx)
		if err != nil {
			return fmt.Errorf("connect for original return: %w", err)
		}
		defer fresh.Close()
		if err = session.ReturnOriginal(ctx, fresh); err != nil {
			return fmt.Errorf("restore original: %w", err)
		}
	}
	if err := session.FinishProgram(ctx); err != nil {
		return fmt.Errorf("finish original return: %w", err)
	}
	for i := 0; i < 2; i++ {
		if err := verifyOmronOriginal(ctx, dial, cfg, session); err != nil {
			return fmt.Errorf("verify original on fresh connection %d: %w", i+1, err)
		}
	}
	return nil
}

func verifyOmronOriginal(ctx context.Context, dial func(context.Context) (*fins.Client, error), cfg commission.ProgramSessionConfig, session *commission.ProgramSession) error {
	client, err := dial(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	cpu, err := client.Identity(ctx)
	if err != nil {
		return err
	}
	if !bytes.Equal(cpu.Raw, cfg.ExpectedCPUData) {
		return errors.New("PLC identity differs")
	}
	status, err := client.Status(ctx)
	if err != nil {
		return err
	}
	if status.Mode != byte(fins.CPUModeProgram) || status.Executing() || status.Fatal != 0 || status.NonFatal != 0 {
		return errors.New("PLC is not in clean PROGRAM mode")
	}
	original, err := client.ReadProgram(ctx, um.MaxSize)
	if err != nil {
		return err
	}
	if !bytes.Equal(original, cfg.ExpectedOriginal) {
		return errors.New("resident program differs from the pinned original")
	}
	return session.VerifyOriginalFiles(ctx, client)
}
