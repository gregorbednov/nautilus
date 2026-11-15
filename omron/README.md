# Omron CJ1M / CJ1H backend — implementation in progress

The working pieces are independent of Windows, Wine, CX-One and vendor DLLs:

* `cxp`: bounded PKWARE DCL decoding; complete CXP/CXT assignment trees;
  Windows-1251, Windows-1252 and UTF-8; lossless original/CXT preservation;
  canonical CXT writing and literal-token CXP packing; textual rung edits.
  Program-scoped symbol inventories retain global/local declaration types,
  addresses and locations, including unqualified auto/FB entries.
  `PlanFBCall` inspects the observed seven-cell scalar call layout, matches pins
  by row to declaration order, resolves source symbols and correlates the saved
  global instance/definition/version. Numeric declaration types remain raw;
  this is source planning, not graph compilation or memory allocation.
  PH_TYPE identities are now confirmed from PHeadDoc metadata and the actual
  CX-P serialization chain: BOOL0, INT1, UINT2, WORD7, TIMER13. Known codes have
  a display name; unknown codes remain raw. Identifying a type does not qualify
  all its operations or complete project-level typing.
* `il`: independent lexical instruction model: source spelling, mnemonic,
  leading `@`/`%`, decimal function label and opaque operands (including quoted
  text). `inspect` exposes parsed instructions and source-located syntax errors;
  malformed instructions remain in the report. Parsing does not establish CPU
  support, operand validity or machine encoding.
* `codec`: initial native encoding/decoding for ten plain Boolean instruction
  forms plus NOP/END, five branch/interlock forms, LD/OUT temporary relays,
  direct-word/literal MOV, and six mode-aware timer/counter
  forms, with distinct typed bit/word
  addresses and radix-preserving literals. Verified against 430 initial and
  370 MOV library observations, 840 timer operand observations and existing
  project anchors. Timers require explicit BCD/binary mode. It handles individual
  containers under library-default profiles; full program compilation and
  unit-version qualification remain outstanding. See [codec limits](codec/README.md).
  Six input instructions and SET/RSET also support qualified @/% differentiation,
  verified against908 additional observations. OUT differentiation stays rejected
  according to the documented instruction variations.
* `compiler`: structured compilation of explicitly selected textual rungs, with
  logic-block/TR/interlock checks, atomic errors, source maps and big-endian
  instruction bodies. The CLI selects contiguous rungs in one CXP/CXT program;
  graphical FBs and unknown instructions fail. See [scope and usage](compiler/README.md).
  Exact named operands resolve from program locals over resource globals, with
  typed direct bindings and per-use provenance. Nine original symbolic rungs
  per CPU match complete sequences in the captured native task bodies.
  `compile-program-body` validates and compiles every declared source rung of
  one program, requiring terminal END. Both original second programs emit 133
  containers matching all source-instruction ranges in the saved tasks; six
  additional captured containers were initially unexplained. This is not native task
  generation. All 41 textual rungs per project compile individually; BLINK calls
  remain explicit blockers for the first program.
  They are now qualified section separators: `-section-markers` emits all 139
  containers matching the second task's complete code, with generated-word
  provenance. Task headers/wrappers and complete N1 generation remain pending.
  The codec also supports plain unsigned-word CMP with two read operands,
  verified against 212 new library observations.
* `fins`: UDP and TCP with node negotiation, strict response correlation,
  direct Host Link FA framing and Linux serial configuration; identity,
  CPU status, raw memory reads and bounded opaque user-memory backups.
* `monitor`: typed, grouped memory polling and the existing Nautilus HTTP/SSE
  API. Values survive an outage with Stale quality. The HTTP facade is read-only.
* `um`: bounded N1 compiler-image inspection: task directory, packed task names,
  entry/exit wrappers and opaque instruction containers. Unknown register/FB
  data and all input bytes are preserved. Supported layout is verified against
  matching CJ1M/CJ1H compiler captures; this is not a full native decompiler.
  `inspect-image -function-blocks` now inventories the observed N1 definition
  and instance records, including opaque code and retained headers. Both saved
  BLINK instances match complete runtime buffers. See [FB evidence and limits](../docs/design/omron-fb-format.md).
  `inspect-image -decode-fb-bodies -model MODEL -timer-mode bcd` also decodes
  the observed relative BLINK body forms independently, including signed12-bit
  BR0/BR3 displacements and instruction spans. This is a bounded decoder;
  relative encoding, allocation and definition compilation remain pending.
* `internal/fbcode`: qualified invocation postamble operations and a frame
  assembler for already encoded/allocated transfer instructions. It matches all
  26 containers of the saved BLINK call and supports the restricted standalone
  call-body compiler.
  Scalar BOOL/WORD/UINT/INT input/output transfers can now be generated from
  typed source operands, verified against176 new native observations. The full
  call test uses generated transfers. Source planning now supplies port order.
  `ScalarTransferForPin` now derives BOOL/INT/UINT/WORD from source declarations
  and checks scalar binding metadata. The27-container LD+BLINK test no longer
  supplies a manual transport-type map. FBICALL construction and caller-state
  choice were initially explicit controls. InstanceCall is now qualified by122
  new observations, and the stock caller's false option is confirmed in Ghidra.
  `compile-fb-call-body` independently emits all27 containers of the original
  LD+BLINK rung, with source/generated spans. It does not generate the referenced
  FB definition or instance records; full-program graph compilation stays closed.

**There is no full-project CJ1 executable compiler or deploy command yet.** Import
bundles are source/research artifacts, not runnable Nautilus projects. The full
Omron instruction palette, graphical FB lowering, native binary decompiler and
deployment verification are still required. No physical controller has been
used to qualify this code. [Research and remaining work](../docs/design/omron-cj1.md).

[План ревизии, дальнейшей реализации и верификации](../docs/design/omron-revision-and-delivery-plan.md)
defines the acceptance gates for the complete independent Linux backend.
[R0 revision results](../docs/design/omron-revision-r0.md) record the current
fixes, portable evidence and remaining verification gaps.

## Commands

From the repository root, build the usual CLI (`CGO_ENABLED=0 go build -o bin/naut ./cmd/naut`).
Flags precede positional arguments, as elsewhere in the CLI.

```sh
bin/naut omron inspect omron/cxp/testdata/cj1m.cxp
bin/naut omron inspect-fb-call -program 'Resource[0].Programs.Program[0]' \
  -rung 'Resource[0].Programs.Program[0].Sections.Sec[0].ProgramData.R[4]' \
  omron/cxp/testdata/cj1m.cxp
bin/naut omron unpack omron/cxp/testdata/cj1m.cxp > /tmp/project.cxt
bin/naut omron import -o /tmp/lift-import omron/cxp/testdata/cj1m.cxp
bin/naut omron inspect-image -format n1 omron/um/testdata/cj1h-baseline.n1
```

The import creates `original.cxp`, `source.cxt`, `project.json` and `report.json`
in a **new** directory. The original bytes and text encoding remain available;
JSON strings are Unicode. `-encoding auto` chooses valid UTF-8, otherwise
Windows-1251, matching the supplied Russian projects. Specify `windows-1252`
for an ANSI Western European project. Encoding is recorded in `project.json`.

Rung paths come from `inspect`. This example replaces only the existing text
payload; graphical FB cells, settings, tasks and all other fields are preserved:

```sh
bin/naut omron rewrite \
  -rung 'Resource[0].Programs.Program[0].Sections.Sec[0].ProgramData.R[0]' \
  -source /tmp/rung.txt -o /tmp/edited.cxp omron/cxp/testdata/cj1m.cxp
```

`rung.txt` contains UTF-8 Omron mnemonics. Graphical-only rungs are refused by
this command. A rewritten CXP uses valid, unoptimized DCL literal tokens, so it
can be larger than the original. Its DCL stream has been checked against the
independent zlib `blast` decoder; CX-Programmer acceptance still needs a Windows
test. Writing a source container does **not** compile it for a CPU.

Compile the selected rewritten textual rung into a JSON code-body report with:

```sh
bin/naut omron compile-rungs -model 'CJ1M CPU21' \
  -rung 'Resource[0].Programs.Program[0].Sections.Sec[0].ProgramData.R[0]' \
  /tmp/edited.cxp
```

Add `-timer-mode bcd|binary` for timers/counters. Repeat `-rung` for a contiguous
selection in one program; project order is preserved. This produces containers
and source maps, not a full N1 image or PLC transfer. See [compiler limits](compiler/README.md).
The command also accepts unchanged supported symbolic rungs; for example select
section0/rung3 from the original CJ1M fixture with `-timer-mode bcd`. Symbol
bindings are audited in the report. UP(521), DOWN(522) and NOT(520) are also
supported; UP/DOWN have198 additional library observations. Unqualified
instructions and graphical FBs still fail instead of being skipped.

## Read-only commissioning

The endpoint and FINS route are explicit; a project file's IP is never contacted
automatically. Replace the example addresses and node numbers with your bench.

```sh
bin/naut omron status -endpoint 192.0.2.10:9600 -source-node 11 -node 10
bin/naut omron read -endpoint 192.0.2.10:9600 -source-node 11 -node 10 D10 2
bin/naut omron upload -endpoint 192.0.2.10:9600 -source-node 11 -node 10 \
  -o /tmp/user-memory.bin
```

TCP (`-transport tcp`) negotiates nodes. `-network`/`-unit` set the destination
route; source and destination FINS nodes need not match the last IP octet.
Serial connects to a port already configured for **Host Link**, not Toolbus:

```sh
bin/naut omron status -transport hostlink -serial /dev/ttyUSB0 \
  -station 0 -baud 9600 -bits 7 -parity even -stop-bits 2
```

Address notation: `CIO100.01` (also `100.01`), `W0.00`, `H512`, `A448`, `D10`,
`E2_100`, `T0`, `C0`. `T0.Q`/`C0.Q` read completion flags; bare `T0`/`C0` read
present values. Wire-family range validation is not a CPU capability check.

## Live dashboard

Bindings are explicit JSON; no symbol browse or automatic address allocation
is inferred from a FINS connection:

```json
[
  { "name": "Start", "address": "W0.00", "type": "BOOL" },
  { "name": "Speed", "address": "D10", "type": "REAL" }
]
```

```sh
bin/naut omron serve -endpoint 192.0.2.10:9600 -source-node 11 -node 10 \
  -tags /tmp/bindings.json -addr localhost:8080
```

`GET /api/state`, `/api/meta`, `/api/stream` and `/api/omron/status` report
readings, quality and CPU status. All HTTP mutations are forbidden. Physical PLC
source is not replaced by the monitor's empty host program. The scan diagnostics
describe **host polling**, not the remote CPU scan.

Supported scalar types: BOOL, WORD/UINT, INT, DWORD/UDINT, DINT, REAL. CJ double
words are decoded low word first, with high byte first within each FINS word.
BOOL bits sharing a word share a read; adjacent reads merge up to 256 words.
Rejected ranges become Bad independently; connection failures hold delivered
values as Stale, with bounded reconnect backoff. Non-finite REALs are Bad.

## Verification

```sh
go test ./omron/... ./cmd/naut
go test -race ./omron/...
```

The network integration tests need local TCP/UDP sockets. They use loopback and
`net.Pipe`, never a real PLC. Source fixtures cover both supplied projects,
including the graphical BLINK FB and its call. Parser fuzz targets are included.
