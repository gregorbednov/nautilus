package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/fins"
	"github.com/joyautomation/nautilus/omron/monitor"
)

const omronUsage = `naut omron — independent CJ1 source tools, rung compilation and read-only commissioning

  naut omron inspect [-encoding auto|utf-8|windows-1251|windows-1252] FILE
      JSON CPU/source inventory, including graphical and function-block rungs.
  naut omron unpack [-encoding ...] FILE
      Write the exact uncompressed CXT bytes to stdout.
  naut omron import [-encoding ...] -o NEW-DIRECTORY FILE
      Preserve original, CXT, parsed tree and inspection report. This is a
      source bundle, not yet an executable Nautilus project.
  naut omron rewrite -rung PATH -source MNEMONICS-FILE -o NEW.cxt|NEW.cxp FILE
      Replace one existing textual rung; preserve all other project fields.
      Source/container editing only: does not compile or deploy to a CPU.
  naut omron inspect-image [-format n1] FILE
      Inspect a captured native N1 compiler image: tasks and code sizes.
      Preserve unknown data; does not decompile, compile or deploy it.
  naut omron compile-rungs -model MODEL [-timer-mode bcd|binary] -rung PATH FILE
      Compile selected textual CXP/CXT rungs into JSON instruction containers.
      Repeat -rung for a contiguous selection in one program (project order).
      Resolve exact local/global names and audit their typed address bindings.
      Partial code body only; not a complete project, N1 image or PLC payload.
  naut omron compile-program-body -model MODEL [-timer-mode bcd|binary] -program PATH FILE
      Compile every source rung in the named program, requiring terminal END.
      Validate section/rung counts and order; reject graphical or unknown code.
      Default output contains source instructions only.
      Add -section-markers for qualified separators, with generated-word spans.
      Task headers, wrappers and the complete N1 image remain pending.
  naut omron status CONNECTION-FLAGS
      Read CPU identity and status; never change its mode.
  naut omron read CONNECTION-FLAGS ADDRESS [COUNT]
      Read raw memory (e.g. D10, W0.01, T0.Q); output JSON hex bytes.
  naut omron serve CONNECTION-FLAGS -tags BINDINGS.json [-addr localhost:8080]
      Live read-only dashboard/SSE; BOOL, WORD, INT, DWORD, DINT and REAL.
      Bindings: [{"name":"Start","address":"W0.00","type":"BOOL"}]
  naut omron upload CONNECTION-FLAGS -o NEW-FILE [-limit BYTES]
      Back up opaque user memory. Does not decompile it or write to the PLC.

CONNECTION-FLAGS:
  -transport udp|tcp|hostlink  (default udp)
  -endpoint HOST:9600         (Ethernet; no default host)
  -source-node N -node N      (required for UDP; TCP can negotiate nodes)
  -network N -unit N          (destination FINS network and unit, default 0)
  -serial /dev/ttyUSB0 -station 0 -baud 9600 -bits 7 -parity even -stop-bits 2
  -timeout 2s                (per exchange; no automatic retries)

Full-project executable generation, deploy and full LD editing are not implemented
yet. No Windows/vendor DLL dependency; physical hardware is not yet qualified.
`

func runOmron(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Print(omronUsage)
		return 0
	}
	var err error
	switch args[0] {
	case "inspect", "unpack", "import", "rewrite":
		err = omronSource(args[0], args[1:])
	case "inspect-image":
		err = omronImage(args[1:])
	case "compile-program-body":
		err = omronCompileProgramBody(args[1:])
	case "compile-rungs":
		err = omronCompileRungs(args[1:])
	case "status", "read", "upload", "serve":
		err = omronRead(args[0], args[1:])
	default:
		err = fmt.Errorf("unknown command %q\n%s", args[0], omronUsage)
	}
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "naut omron:", err)
		return 1
	}
	return 0
}

func readOmronFile(name string) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, cxp.MaxSize+1))
	if err != nil {
		return nil, err
	}
	if len(b) > cxp.MaxSize {
		return nil, errors.New("project exceeds size limit")
	}
	return b, nil
}

func omronSource(command string, args []string) error {
	f := flag.NewFlagSet("omron "+command, flag.ContinueOnError)
	encoding := f.String("encoding", "auto", "CXT text encoding; auto falls back to Windows-1251")
	output := f.String("o", "", "new directory for import")
	rung := f.String("rung", "", "fully qualified textual rung path")
	source := f.String("source", "", "UTF-8 mnemonic source file for rewrite")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 1 {
		return errors.New("expected one CXP/CXT filename (flags precede the filename)")
	}
	data, err := readOmronFile(f.Arg(0))
	if err != nil {
		return err
	}
	d, err := cxp.ReadEncoding(data, *encoding)
	if err != nil {
		return err
	}
	switch command {
	case "rewrite":
		if *output == "" || *rung == "" || *source == "" {
			return errors.New("rewrite needs -o NEW.cxt|NEW.cxp, -rung PATH and -source FILE")
		}
		text, err := readOmronFile(*source)
		if err != nil {
			return err
		}
		if err := d.SetRung(*rung, string(text)); err != nil {
			return err
		}
		out, err := d.MarshalCXT()
		if err != nil {
			return err
		}
		switch strings.ToLower(filepath.Ext(*output)) {
		case ".cxp":
			out, err = cxp.Pack(out)
		case ".cxt":
		default:
			return errors.New("rewrite output extension must be .cxp or .cxt")
		}
		if err != nil {
			return err
		}
		return writeNewOmronFile(*output, out)
	case "unpack":
		_, err = os.Stdout.Write(d.Source)
		return err
	case "import":
		if *output == "" {
			return errors.New("import needs -o NEW-DIRECTORY")
		}
		if err := cxp.Export(d, data, *output); err != nil {
			return err
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(d.Summary())
}

func omronRead(command string, args []string) error {
	f := flag.NewFlagSet("omron "+command, flag.ContinueOnError)
	tags := f.String("tags", "", "JSON monitor bindings")
	addr := f.String("addr", "localhost:8080", "local read-only HTTP server address")
	poll := f.Duration("poll", 250*time.Millisecond, "monitor polling period")
	stale := f.Duration("stale-after", 2*time.Second, "maximum age of a good tag value")
	transport := f.String("transport", "udp", "udp, tcp or hostlink")
	endpoint := f.String("endpoint", "", "Ethernet HOST:PORT")
	src := f.Int("source-node", 0, "source FINS node")
	node := f.Int("node", 0, "destination FINS node")
	network := f.Int("network", 0, "destination FINS network")
	unit := f.Int("unit", 0, "destination FINS unit")
	serial := f.String("serial", "", "Linux serial device")
	station := f.Int("station", 0, "Host Link station 0..31")
	baud := f.Int("baud", 9600, "serial baud")
	bits := f.Int("bits", 7, "serial data bits")
	parity := f.String("parity", "even", "serial parity")
	stop := f.Int("stop-bits", 2, "serial stop bits")
	timeout := f.Duration("timeout", 2*time.Second, "per-exchange timeout")
	output := f.String("o", "", "new backup file")
	limit := f.Int("limit", 4<<20, "maximum program backup bytes")
	if err := f.Parse(args); err != nil {
		return err
	}
	for _, v := range []int{*src, *node, *unit} {
		if v < 0 || v > 254 {
			return errors.New("FINS node/unit must be 0..254")
		}
	}
	if *network < 0 || *network > 127 || *timeout <= 0 {
		return errors.New("invalid network/timeout")
	}
	var memory fins.Memory
	count := 1
	if command == "read" {
		if f.NArg() < 1 || f.NArg() > 2 {
			return errors.New("read requires ADDRESS [COUNT]")
		}
		var err error
		memory, err = fins.ParseMemory(f.Arg(0))
		if err != nil {
			return err
		}
		if f.NArg() == 2 {
			var err error
			count, err = strconv.Atoi(f.Arg(1))
			if err != nil {
				return err
			}
		}
		if count < 1 || count > 996 {
			return errors.New("count must be 1..996")
		}
	} else if f.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if command == "upload" && (*output == "" || *limit < 4 || *limit > 32<<20) {
		return errors.New("upload requires -o NEW-FILE and a byte limit 4..33554432")
	}
	route := fins.Route{Source: fins.Address{Node: byte(*src)}, Destination: fins.Address{Network: byte(*network), Node: byte(*node), Unit: byte(*unit)}}
	ctx, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()
	var dial monitor.Dial
	switch *transport {
	case "udp", "tcp":
		if *endpoint == "" {
			return errors.New("Ethernet needs -endpoint HOST:PORT")
		}
		if *transport == "udp" && (*src == 0 || *node == 0) {
			return errors.New("UDP needs explicit -source-node and -node")
		}
		dial = func(ctx context.Context) (*fins.Client, error) {
			if *transport == "udp" {
				return fins.DialUDP(ctx, *endpoint, route, *timeout)
			}
			return fins.DialTCP(ctx, *endpoint, route, *timeout)
		}
	case "hostlink":
		if *serial == "" || *src != 0 || *node != 0 || *network != 0 || *unit != 0 || *station < 0 || *station > 31 {
			return errors.New("direct Host Link requires -serial, station 0..31 and a local FINS route")
		}
		dial = func(ctx context.Context) (*fins.Client, error) {
			file, err := fins.OpenSerial(*serial, fins.SerialConfig{Baud: *baud, DataBits: *bits, Parity: *parity, StopBits: *stop})
			if err != nil {
				return nil, err
			}
			client, err := fins.NewHostLink(file, *station, 0, *timeout)
			if err != nil {
				_ = file.Close()
			}
			return client, err
		}
	default:
		return fmt.Errorf("unknown transport %q", *transport)
	}
	if command == "serve" {
		return serveOmron(ctx, dial, *tags, *addr, *poll, *stale)
	}
	client, err := dial(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	switch command {
	case "status":
		id, err := client.Identity(ctx)
		if err != nil {
			return err
		}
		status, err := client.Status(ctx)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(struct {
			CPU    fins.Identity `json:"cpu"`
			Status fins.Status   `json:"status"`
		}{id, status})
	case "read":
		data, err := client.ReadMemory(ctx, memory, uint16(count))
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(struct {
			Address string `json:"address"`
			Count   int    `json:"count"`
			Hex     string `json:"hex"`
		}{f.Arg(0), count, hex.EncodeToString(data)})
	case "upload":
		data, err := client.ReadProgram(ctx, *limit)
		if err != nil {
			return err
		}
		return writeNewOmronFile(*output, data)
	}
	return nil
}

// Link the completed temporary file into place without replacing an existing
// artifact. A failed write never leaves a partial file at the requested path.
func writeNewOmronFile(name string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(name), ".omron-artifact-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Link(f.Name(), name)
}
