# CJ1M/CJ1H reverse engineering and implementation record

Current revision and delivery roadmap (2026-11-14):
[план ревизии, реализации и верификации](omron-revision-and-delivery-plan.md).
The entries below retain the investigation history; pending actions in older
entries may have been superseded by the current findings and roadmap.

The required endpoint is a native Omron target in Nautilus: all instructions
valid for the selected CPU/unit version, extended LD editing, full import,
native compilation, verified full download/upload and live monitoring. Production
compilation and communication must remain independent of Windows and CX-One.
Online edit, Toolbus and full local emulation of CJ1 hardware are outside v1.

## Observed artifacts

The workspace contains two lift projects and a ZIP distribution of CX-One
v4.40, with 4,597 entries. `tools/omron-research/inspect_cxone.py inventory ZIP`
hashes the distribution and inventories the relevant CABs and CPU profiles.
The proprietary binaries are research inputs; they are not shipped in this repo.
The supplied ZIP SHA-256 is
`f8f41809c4d7c661ddad7b3d54c2d5a0bc170c4ef2eff8856dc693bf68938af1`.

Both projects are PKWARE DCL streams (literal mode 0, dictionary parameter 6),
with no wrapper before the stream and no trailing bytes after the end marker.
Their expanded data starts with the CXT assignment syntax `CXProgVer:=`.

* CJ1M: Config declares `DEV:CJ1M;CPU:CPU21`. **Its PLC Settings payload still
  names CJ1H-H CPU65.** This inconsistency is reported, not repaired implicitly.
* CJ1H: Config declares `DEV:CJ1H-H;CPU:CPU65`.
* Each has two program tasks and a BLINK function block. In the CJ1M fixture,
  the source inventory has 48 nonempty rungs: 231 textual instruction lines
  plus seven graphical rungs (six in the FB definition and one FB call).
* Text payload delimiters `$?St$Bk?_#[n]` are paired markers, **not lengths**.
  Variable lists use separate `BEGIN_LIST_$#[n]` / `END_LIST_$#[n];` markers.
* Normal ladder text lives in `SL`; FB bodies use `FBR[n]` cell records, while
  a program's graphical FB call uses control-character-separated cell records.
  Converting only `SL` would silently omit executable logic.

Complete trees and original bytes are preserved. Canonical CXT and DCL-packed
CXP round trips retain unknown fields and graphical cells. Literal-token CXP
packing was independently decoded with upstream zlib `contrib/blast` and
compared byte-for-byte. This establishes container interoperability, not
acceptance by CX-Programmer or a physical CPU.

## Compiler investigation

`CXPCore.cab` SHA-256:
`8b33820751c4708c40bd272cbcdfb70052851ec26f38cfb75557a89268a9669f`.

The contained x86 `cx_pdata.dll` SHA-256:
`a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141`.

Static exports and disassembly identify:

* `CInstruction` constructor at VA `12012ec0`;
* `InstructionInst::Compile` at `1216b407` / `1216b432`;
* `PLC_Program::Compile` at `121e3eef`;
* constructor stack offsets `4c`/`50` stored into object offsets `c0`/`c1`;
  the exported operand-count getter reads `c0`;
* name construction through the CString wrapper at `1224e2de`;
* 1,505 constructor call sites, including one dynamic copy path; the remaining
  **1,504 static records** yield names, operand counts, raw function identifiers,
  raw family masks and handler pointers. They contain 939 distinct public
  spellings across multiple Omron families, not 939 qualified CJ1 instructions.

Reproduce after extracting the two CABs into a temporary research directory:

```sh
python3 tools/omron-research/inspect_cxone.py compiler PATH-TO-cx_pdata.dll \
  --out /tmp/compiler-facts.json
```

The extractor requires the exact researched SHA-256 and verifies its expected
record count. It never runs vendor code. It deliberately preserves unknown
metadata as raw numbers: function identifiers are not asserted to be machine
opcodes, family masks are not yet asserted to identify particular CPUs, and
handler addresses are not used by the Go backend.

## Implemented communication

Protocol reference: Omron [W342-E1-18](https://files.omron.eu/downloads/latest/manual/en/w342_cs_cj_cp_nsj_communications_commands_reference_manual_en.pdf),
sections 3-5 and 5-3. Programming/instruction semantics must also be checked
against [W394](https://assets.omron.eu/downloads/latest/manual/en/w394_cs1_cj1_nsj_series_programmable_controllers_programming_manual_en.pdf)
and W340 for the selected CPU and unit version.

The Go FINS client implements UDP, negotiated TCP and directly connected Host
Link FA framing, plus Linux tty configuration. Read-only operations cover CPU
data/status, explicit memory ranges and opaque UM blocks (`0306`). Responses
validate route, SID, command, end code, echoed program address and transfer
length. Program transfers require the final-block flag and obey a byte limit.

An I/O failure or malformed response poisons the session; callers reconnect
explicitly. PLC end codes remain typed errors. No uncertain write is retried,
no program is cleared and no CPU mode is changed by the supplied CLI commands.
The monitor reconnects only for read operations and carries per-tag quality.

## Remaining work and acceptance gates

1. **Windows oracle:** install the supplied CX-One and record its component
   versions. Open both unchanged fixtures; save CXT and compiled UM images.
   Record the resolved CPU model/unit version, task IDs, FB definitions,
   instance allocation and actual I/O configuration. Resolve the inconsistent
   CJ1M Settings header explicitly before any deployment.
2. **Instruction/CPU matrix:** map the recovered family/version masks and
   operand forms to CJ1M/CJ1H profiles. Obtain minimal differential examples for
   each instruction form, modifiers, addressing modes and boundary operands.
   Recognition, encoding, oracle acceptance and hardware behavior are separate
   statuses; the complete matrix is the acceptance target, not a basic subset.
3. **Native codec:** independently implement UM/task envelopes, instructions,
   operands, FB instances, relocation/allocation, checks and decoding. Compare
   against CX-generated images and negative cases before enabling deployment.
   Do not infer machine code from mnemonic function numbers alone.
4. **Nautilus target/editor:** add target/dialect configuration, the full native
   instruction model and palette, graphical FB lowering, CPU diagnostics,
   address allocation and shared CLI/LSP rules. Imported raw trees are not a
   substitute for this step. Reject constructs without correct lowering.
5. **Deployment:** preflight exact hardware/version, preserve an on-device
   backup and affected settings, show changes, enter PROGRAM only explicitly,
   transfer, read back and compare, then enter RUN only explicitly. Keep UM,
   PLC settings, I/O tables and retained data separate. An interrupted transfer
   is uncertain until reconciled against the PLC, not automatically rolled back.
6. **Hardware qualification:** perform the full cycle on CJ1M and CJ1H using
   Ethernet and Host Link, including power/reconnect interruptions, protected
   memory and read-back failures. Instructions involving special modules need
   those modules. Final acceptance cannot be replaced with an emulator run.

For every oracle case retain original and changed CXP/CXT, compiled UM images,
the single changed feature, tool/CPU/unit versions, communication traces when
applicable, expected results and SHA-256 hashes. Keep vendor tools in the
research harness; do not introduce a production Windows agent.

## Windows continuation: lexical instruction model

Added `omron/il`, a vendor-independent representation of textual mnemonic
instructions. It retains original spelling, leading @/% modifier, optional
decimal function label (as text, preserving leading zeroes) and opaque operand
spellings. Quoted operands can contain spaces and doubled quotes. Errors carry
a one-based UTF-8 byte column. CXP inspection now attaches this representation
or a syntax diagnostic to each textual instruction without discarding source.
Both original fixtures are covered, including preservation of graphical rungs.

This is a lexical front end, not a CPU instruction schema. Unknown mnemonic
spellings remain representable; neither function labels nor static DLL function
identifiers are treated as opcodes. Operand counts/types, CPU/unit applicability,
graphical lowering and executable encoding remain unresolved acceptance gates.
The Windows COM probe's zero operand counts for LD/OUT are not used as a schema.

Validation on Windows: `go test ./omron/...`, `go vet ./omron/...` and
`go test ./cmd/naut -run Omron -timeout 60s` passed. A 10-second IL fuzz run
passed with 17,741 executions. The full CLI suite is not green on this host:
`TestBuiltControllerActivatesWarm` cannot execute the generated extensionless
`naut`; a golden diagnostic expects Unix path separators; and the bounded run
timed out in `TestCheckReportsStructureWarnings`, writing to the capture pipe.
These failures are outside the Omron-specific tests and were not modified.

## Reproducible native-format research on Windows

Ghidra-based selected-function analysis and a statically reconstructed template
catalog are now available. All 1504 constructor records matched the prior facts;
30 template/base-length differences remain explicitly flagged. The whole-project
source corpus generator produced 14 cases per supplied CPU project and verifies
that all fields beyond the chosen rung are preserved. See
[the native encoding findings](omron-native-encoding.md) and
[research harness usage](../../tools/omron-research/README.md).

The environment's desktop helpers failed during initialization. CX-Programmer
GUI acceptance and compiled UM capture have therefore not been performed.
The native compiler, full instruction/CPU qualification and deployment remain
outstanding; no claim of completion or hardware acceptance is made.
