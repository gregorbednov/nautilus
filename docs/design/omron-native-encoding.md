# Native encoding findings from the installed Windows compiler

## Selected-rung structure and code-body compilation

Independent `omron/compiler` composes qualified containers after checking
explicit network boundaries, condition depth, TR definitions and IL/ILC regions.
It returns source spans and no partial body on any error. It does not insert END;
an explicit END must be terminal. CNT/CNTX require count/reset conditions and
terminate their selected block; continuation from counter inputs is unqualified.
Multiple ILs sharing one ILC are accepted with a diagnostic, not treated as a
nested one-to-one stack. Logic depth is bounded at8; duplicate TR stores inside
a block and references across network/new-block boundaries are rejected.

The CXP adapter reparses current textual instructions, refuses graphical FBs,
and never relies on stale cached Parsed instructions. CLI `omron compile-rungs`
requires explicit model and contiguous selected rungs in one native program,
retaining project order. Its report contains bare big-endian containers, source
hash, source spans and false full-project/hardware qualification fields.

Controlled first-rung replacements in the two original CXP fixtures match the
first two task containers in the saved H-baseline/M-input-bit N1 images using
the respective CPU profiles. The actual CLI compiled the existing M-input-bit
experiment to e205e00ffa05e001 with sourceSHA
becb4b4c39cad9f8a336ecc61a91957923e69ee46c990effca9125b9eb49badf.
Complex structure rules have manual/test coverage, not a new whole-program
CX ProgramCheck capture. Graph lowering, symbols, FB allocation and complete
native images remain outstanding. See [compiler scope](../../omron/compiler/README.md).

## Differentiated bit instructions

A new454-input-per-CPU capsule records908 observations: Go matches496 supported
encodings, rejects176 invalid cases and leaves236 forms outside documented CPU
or qualified operand/instruction scope. All492 common results reproduce in the
previous complete cold worker. Supported forms are @/% LD, LDNOT, AND, ANDNOT,
OR, ORNOT, SET and RSET; existing direct-address access and CJ1 DM-bit exclusions
are reused rather than bypassed by the modifier path.

Input variants use `e2c00000 | bit-field`, then `cb800000` for @ or `cba00000`
for %, then a fixed suffix: LD=c2125002, LDNOT=c2325002, AND=c2525002,
ANDNOT=c2725002, OR=c2925002, ORNOT=c2b25002. SET/RSET use prefixfac00000 (@)
orfae00000 (%) and second containerd4400000|field (SET) ord4600000|field (RSET).
Lengths/tags/suffixes are exact; malformed or unqualified forms consume zero.

The reviewed actual Ghidra artifact `research/cj1-oracle/ghidra-n2.json` includes
input encoder120662e3 and output encoder120c8ec7. Assembly explicitly writes
cb800000 at12066773, cba00000 at12066761, and temporary operand125002 at12066838;
output prefixes are written at120c9031 and120c905a. These are fixed literals,
not a caller-selected allocation field in the qualified direct forms.
Separate investigation artifacts `ghidra-edge-encode.json` and
`ghidra-edge-handler.json` cover dispatch and a decoder; do not confuse decoder
120660c0 (constructor argument39) with the encoder at argument40.

Whole-project captures contain two e2d20000,cb800000,c2525002 triples each.
Only their helper/suffix corroborate this pattern: operand120000 is an unknown
internal field, not a qualified CIO/W/H/A/T/C address. The entire triple stays
opaque on decoding; no full modified-instruction project anchor is claimed.

W474 lists @/% contacts and SET/RSET, but only immediate-refresh variations for
OUT/OUT NOT. The low-level library still emits %OUT and @/%OUTNOT, so all edge
OUT forms remain rejected. Modified TR, MOV, timers and branch forms remain
unqualified; in particular %MOV and %ANDLD can silently emit their plain form.
Source: [Omron W474](https://assets.omron.eu/downloads/latest/manual/en/w474_cs_cj_nsj-series_plc%2C_sysmac_one_nsj_series_reference_manual_en.pdf?v=9).

## Branch and temporary-relay containers

The branch capsule records262 inputs per CPU (524 observations): independent Go
matches86 supported outputs, rejects386 invalid forms and leaves52 modified,
incorrect-label or foreign-mnemonic controls unsupported. All188 common results
from the earlier cold run reproduce, including accepted status, initialization,
operand errors and generated words.

| Form | Numeric container |
| --- | --- |
| NOT | e3000000 |
| ANDLD | e3200000 |
| ORLD | e3400000 |
| IL / IL(002) | b1000002 |
| ILC / ILC(003) | bb000003 |
| LD TRn | e2122000 + n |
| OUT TRn | fa122000 + n |

All five branch/interlock instructions take zero operands. TemporaryRelay is
a separate Go operand type, with indices0..15, accepted only by LD and OUT.
All16 slots and invalid access forms were measured; TR16 and dotted TR operands
fail. Decoder rejects reserved relay fields, wrong operations and altered
zero-operand containers without consuming input. MPS/MRD/MPP initialization
fails in this library context; no substitute encoding is inferred.

Existing full-project captures corroborate ten ANDLD containers, three ORLD
containers and eight LD/OUT TR0/TR1 containers in each image. These are numeric
anchors; this does not validate an arbitrary branch graph. The selected-rung
compiler now checks initial IL/ILC, block-depth and TR lifetime rules; complete
project context and graph lowering remain outstanding. W474 describes
AND/OR block combination and restricts TR0..TR15 to LD/OUT; see
[Omron W474](https://assets.omron.eu/downloads/latest/manual/en/w474_cs_cj_nsj-series_plc%2C_sysmac_one_nsj_series_reference_manual_en.pdf?v=9).

## CPU restrictions override low-level library success

W474-E1-13 sections3-3-1..3-3-6,3-4-1 and3-4-8 restrict direct DM/EM bit operands
in ordinary Boolean instructions to CJ2. CX-Pdata's instruction-unit path still
emits DM bit containers for CJ1M/CJ1H. Encode/decode now reject DM bit operands
for all ten ordinary Boolean forms; DM word MOV and timer presets are unaffected.
After the DM-bit exclusion the first430-observation capsule gave284 accepted
encodings,80 invalid cases and66 outside supported forms or CPU limits. With
the qualified differentiation forms added it now gives292/80/58 respectively.
The Data bit-field entry below records a library representation, not permission
to use that address in CJ1 Boolean instructions. `ParseBitAddress` preserves this
representation for research but instruction encoding enforces the CPU limit.
Source: [Omron W474-E1-13](https://assets.omron.eu/downloads/latest/manual/en/w474_cs_cj_nsj-series_plc%2C_sysmac_one_nsj_series_reference_manual_en.pdf?v=9).

## Explicit timer/counter instruction codec

The next operand matrix contains210 inputs per CPU in each explicit mode:
840 observations. Independent Go matches252 encodings, rejects540 invalid or
wrong-mode forms, and leaves48 indirect cases unqualified. `Profile` now exposes
`TimerCounterMode`: empty rejects timer/counter encode/decode rather than guessing
the meaning from a shared opcode. Unknown mode strings fail profile validation.

| BCD spelling | Binary spelling | First container | Slot container |
| --- | --- | --- | --- |
| TIM | TIMX(550) | bb4000c0 | 5000e000 + index |
| TIMH(015) | TIMHX(551) | bb4000c1 | 5000e000 + index |
| CNT | CNTX(546) | bb4000d0 | 5000f000 + index |

Indices span0..4095; a matching T/C prefix is accepted. The third container is
`65400000 | #literal`, `65440000 | &literal` (binary mode only), or
`64000000 | direct-word-address`. BCD #literal digits must each be0..9, so #ABCD
fails although the binary form accepts it. Direct preset addresses use the MOV
word-address fields below, including read-only A words as sources. Truncated,
wrong-slot, reserved-tag and unsupported indirect forms consume zero on decode.

Seven numeric timer sequences are also present in each existing N1 task capture,
including `bb4000c0,5000e000,65400100` and
`bb4000c1,5000e006,65400050`. This corroborates emitted containers but does not
establish the captured project's timer mode. FB-relative sequences such as
`bb4000c1,513c0000,653c3001` remain opaque; no allocation or FB lowering is implied.

## R1/R2 continuation: MOV and recovered timer context

The separate MOV capsule records 185 inputs for each CPU: 288 supported outputs
match independent Go, 72 invalid cases fail, and 10 indirect/modifier/label
observations remain unqualified. The 174 common results from the first cold run
are identical. MOV/MOV(021)/MOV(21) encode as three numeric containers:
`b2400140`, source, destination. Literal source tags are `61400000 | uint16`
for #hex and `61440000 | uint16` for &decimal. Direct word sources use
`60000000 | address`; destinations use `54000000 | address`.

| Word area | Address base | Direct index range |
| --- | --- | --- |
| numeric CIO | 0xc000 | 0..6143 |
| W | 0xde00 | 0..511 |
| H | 0xd800 | 0..511 |
| A | 0xb840 | 0..959; writes start at448 |
| D | 0x10000 | 0..32767 |
| T present value | 0xe000 | 0..4095 |
| C present value | 0xf000 | 0..4095 |

Unsigned literals span0..65535; signed forms and bit suffixes on word addresses
are rejected. Unlike Boolean completion operands, T/C present-value destinations
work in this measured MOV context. Reserved tags and address gaps fail decoding
with zero consumed containers. No full-project or hardware qualification follows.

Actual Ghidra assembly review located normal timer-mode APIs: SetTimerCounterModeBcd
at121cf6b3 writes1 to KBase+2a0; SetTimerCounterModeHex at121cf6cb writes0.
IsTimerCounterModeBcd at120125e0 reads that field; IsTimerCounterModeBin at12012600
tests it against0. Research artifact: `research/cj1-oracle/ghidra-timer-mode.json`.
Craft function evidence IDs are
`ev_9cda0fd4b85eea6240c5d89d3b8d550173cd268a021ead08c42ed418f4978a0a`,
`ev_67fc5ebd7a236a904d8ec976f68207ee2cfb24d11f3037550bc2bbb9397b7b47`,
`ev_b7c52ca39fafaf0b7afbe7870053b526131d8b9691cfc9da53a4d46d7dad5986`,
`ev_46a83e2fd5eeb5be5b13bcdd8824c05bb98fe739879b7bf5f5fb96b71081cb81`.

The harness now selects and reads back this mode after each SetPLC. Three
portable capsules preserve84 probes: measured default equals binary; TIM,
TIMH(015), CNT initialize only in BCD, while TIMX(550), TIMHX(551), CNTX(546)
initialize only in binary. BCD forms reject &decimal presets in this probe.
The MOV #/& controls remain unchanged across modes. This resolves the earlier
initialization failure. These initial configuration probes preceded the operand
matrix and timer codec described above; they alone did not qualify timer forms.

## R1: instruction-unit oracle and initial independent codec

A new bounded library harness runs in a disposable x86 PowerShell worker,
without GUI or user projects. Its reviewed API sequence is constructor,
SetPLC, InstructionInst::Initialise, CInstruction::OperandEncode, then
InstructionInst::EncodeFunction. Native destructors run on completion.
The 215 explicit inputs for each of CJ1M CPU21 and CJ1H-H CPU65 yield 430
saved observations. A repeat run reproduced all 172 shared earlier cases.
The corpus includes address boundaries, access restrictions, modifiers,
function-label controls, literals and timer probes.

Independent Go package `omron/codec` now encodes and decodes ten plain Boolean
forms (LD/LDNOT, AND/ANDNOT, OR/ORNOT, OUT/OUTNOT, SET/RSET), NOP and END.
The initial Boolean-only codec matched322 generated results, rejected80
negative cases and left28 observations outside its scope. With MOV added,
the same immutable capsule now gives326/82/22 respectively. Existing
GUI-derived full-project N1 images also anchor LD W0.00/W0.15, OUT W0.01 and END.
The machine containers are not an assembled project or a PLC transfer payload.

Measured direct-bit address ranges use these low operand fields:

| Area/context | Base field | Qualified direct range |
| --- | --- | --- |
| numeric CIO | 0x40000 | word 0..6143, bit 0..15 |
| W | 0x5e000 | word 0..511, bit 0..15 |
| H | 0x58000 | word 0..511, bit 0..15 |
| A | 0x38400 | word 0..959; words below448 reject writes in measured forms |
| D | 0x80000 | library field only; ordinary CJ1 Boolean forms reject DM bits |
| T completion | 0x3e000 | index 0..4095; read operands |
| C completion | 0x3f000 | index 0..4095; read operands |

These limits describe the measured library-default instruction context.
They are separate from FINS addressing and future FB allocation. In native IL
numeric CIO spelling succeeds while CIO0.00 fails in this path. Bare T/C are
completion flags here, whereas the monitor's bare T/C are present values.

The earlier helper at1204535e consumes already normalized operands; it is not
the original text-address mapper. CInstruction::OperandEncode is at12032d70,
InstructionInst::EncodeOperands at1216dc87. Constructor/init/SetPLC functions
and string ABI were reviewed with Ghidra before isolated library calls.
The COM KBaseComp probe produced no observations from CX-Pdata.dll and is not
this oracle.

Qualification limits are material: unitVersion remains null, timer mode is
library-default and no full ProgramCheck, graph/task lowering or relocation
runs. END(999)/LD(999) still produce containers through this low-level path;
TIM/TIMH originally failed initialization under defaults; the mode probes
above explain this failure and preserve the corrected research configuration.
Thus neither source acceptance nor CPU instruction rejection can be inferred
from this path alone. The Go codec rejects unqualified labels/modifiers and
explicit unit-version profiles, and consumes zero words on decode failure.

Portable inputs/events/script snapshots and hashes are documented in
[codec testdata](../../omron/codec/testdata/README.md). Raw research is in
`research/cj1-oracle/instruction-library-03`; no hardware connection occurred.

Validation: Omron/research tests, Omron CLI tests and go vet passed on Windows;
the new decoder fuzz run completed 79,341 executions in five seconds. Linux
ARM64 cross-build passed. Native Linux execution and hardware qualification
remain open; the workflow now also includes the instruction decoder fuzz test.

## Current result: native images captured and independently inspected

The CJ1H-H CPU65 and CJ1M CPU21 baseline projects now have actual compiler
output captures. Both outputs are4592bytes and byte-identical, SHA-256
`8a88c6e726542537787cea1419ccc42a1d203a37efb2304218c2a4e1687ce4f3`.
The CJ1H repeat compile also produced the same hash. Earlier paragraphs below
are a chronological investigation record; their pending-capture statements
have been superseded by these results.

Frida17.26.3 observed the pinned DLL's existing compiler calls, without calling
native compiler functions or changing their arguments/results. The initial
CJ1H capture saw80 encode events, two176-byte FB-related program buffers,
and a successful binary-file compiler return. Its output path was used to
copy the actual temp file. The revised observer captured CJ1M output directly
after compiler return, including263 instruction serialization events and
zero capture errors. Per-instruction encoding alone missed cached ladder code;
the serialized file is the authoritative whole-project artifact.

Observed CPU selection: CJ1M/CPU21 resolves selector8, mask0x400/0; the CJ1H
compile uses selector8, mask0x80000/0. The shared encoder dispatch is now
observed for both examples; this does not qualify every instruction or unit
version. During project initialization default CS1G/CJ1G profiles also appear;
use the compile-enter profile, not the union of every observed profile.

The files use the N1 path, not ProduceN2BinaryFile (whose static guard requires
selector bit0x20). Header and instruction containers serialize big-endian.
The recovered layout has a330-dword header, optional opaque register/FB data,
a288-entry task directory, then task records. Directory entries are dword
offsets relative to the directory start. Header word0x20 is total dword count,
0x21 is0x2a, 0x22 points to the directory; 0x23/0x24 locate auxiliary sections.
Task records have a32-dword header, entry/exit wrapper, and code. Task length
is at header+0x1a; header+0x1b holds32. Names pack three bytes per dword.

Independent Go package `omron/um` now bounds and validates this N1 layout,
finds task256 MotorDrive_LIFT_1 (151 code containers) and task257 NewProgram1
(139 containers), and preserves all original bytes including unknown sections.
It rejects overflowing offsets/lengths, overlaps, missing wrappers and
unexplained code-area gaps. `naut omron inspect-image -format n1 FILE` exposes
the read-only inventory. It is not a complete native decompiler or compiler.

The CJ1M input-bit variant also compiled and was captured: W0.00 -> W0.15
changes exactly byte3283 from00 to0F. The first task's first container becomes
e205e00f from e205e000; image length and every other byte stay unchanged.
Variant SHA-256: `52d783d09159eb2be3a76aa56dcf899bfd7d313da727107bcabe0a416b28eeb5`.
Both binary fixtures and their source/event hashes are in `omron/um/testdata`.
Raw evidence remains under research/cj1-oracle in cj1h-baseline-runtime-01,
cj1m-baseline-runtime-6048 and cj1m-input-bit-runtime-01. The two experimental
sources remain unchanged. No PLC connection or hardware execution occurred.

Next: controlled word/area/modifier comparisons, CPU-qualified operand
encoding, FB/relocation semantics and image generation. The captured compiler
file has not been qualified as a direct PLC transfer payload.

## Provenance and current limits

The researched CX-Pdata.dll SHA-256 remains
`a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141`.
Ghidra 12.1.4 on its installed Java 21 runtime decompiled the selected x86
functions. The reproducible harness is documented in
[the research tools README](../../tools/omron-research/README.md).

Results and vendor-derived disassembly/pseudocode are local research artifacts
under `E:\cx-proger\research\cj1-oracle`, not production source. The canonical
catalog is `catalog.json`; debug/intermediate catalog runs are not inputs to a
compiler. GUI automation could not initialize in this session, so no claim is
made that CX-Programmer has opened, compiled or accepted the generated cases.
No PLC was contacted.

## Confirmed static relationships

- CInstruction constructor: `12012ec0`. Handler arguments are interleaved
  encode/decode pairs. In zero-based constructor stack argument numbering,
  36/38/40 populate object +34/+38/+3C (encoding), and 37/39/41 populate
  +40/+44/+48 (decoding). Treating the first three recovered addresses as
  three encoders would select the wrong code.
- `InstructionInst::EncodeFunction` at `1216d735` dispatches using a PLC
  descriptor field at +44: selector 1 uses +34, selector 2 uses +38,
  selectors 4/8/10/20 hex use +3C. Mapping these selectors and the raw
  family/version masks to named CPU profiles still requires confirmation.
- `CInstruction::GetNumBinWords` at `1203994e` uses +AA, +AB or +AC for
  these selector groups. These are the constructor's zero-based arguments
  9, 10 and 11 respectively.
- Template pointers at CInstruction +10C/+110/+114 originate from zero-based
  arguments 51/52/53. They are dynamically allocated by initialization
  helpers, not always pointers to static PE data.
- Helper `1203a708` takes a count and six values and allocates 32-bit elements.
  Helper `1203a7a3` takes a selector, count and six values; selector 3 creates
  32-bit elements, other selectors create 16-bit elements. Zero count returns
  null. These rules were recovered from each helper and used for static
  reconstruction without executing either helper.
- `InstructionInst::GetBinaryCode` at `1216f468` appends 32-bit container
  elements from object +0C, using the count at +34 when flags at +30 permit
  it. This does not establish the serialized PLC wire byte order.
- `PLC_Program::GetProgramBinary` at `121d68cf` gathers instruction buffers.
  `CompileMultipleProgramsAndProduceBinaryFile` at `121c28ef` reaches
  `ProduceN2BinaryFile` at `121d08e1`. N2 adds headers, offsets and multiple
  sections; concatenating instruction templates is not a complete UM image.

## Catalog recovery and unresolved length differences

All 1,504 static constructor records were reconstructed and checked against
the previous extractor's operand counts, both family masks and all six handler
addresses: zero extraction errors. This still includes multiple PLC families.
Thirty template entries differ from the corresponding declared base length;
the exact differences are retained for investigation. They may involve special
or synthetic instructions; they have not been explained or normalized away.

Example raw templates at +114:

| Constructor call | Spelling | 32-bit template |
| --- | --- | --- |
| `120de227` | LD | `e2000000` |
| `12116249` | OUT | `fa000000` |
| `120a316a` | END | `bb800001` |
| `120f7312` | MOV | `b2000140` |

These are literal initialization templates before operand insertion,
modifiers, relocation and envelope generation. Other records bearing the same
mnemonic can have different templates. No production encoder is enabled by
this table alone.

## Prepared oracle cases

The `corpus` Go tool generated fourteen whole-project source cases per CPU
from the original files in `projects`. Original bytes, CXP/CXT hashes and the
single changed feature are recorded. Tests reverse each edit and compare the
complete canonical tree, including graphical FB cells and task/settings data.

Outstanding next evidence: resolved CPU/unit profiles, CX-Programmer acceptance
of the generated containers, actual compiled binary capture, operand encoding,
N2 section/relocation semantics, and full-project output including BLINK. These
must precede an enabled compiler/deploy path. Hardware qualification remains a
separate gate when a bench is available.

## Validation of this research increment

- Full catalog extraction repeated with identical SHA-256:
  `0bbcc924237c81e85a40e4ca4c5197570bbcf31d38247c6a5e82bae07d53879a`.
  Both runs reported 1504 records and zero extraction errors.
- `go test ./omron/... ./tools/omron-research/corpus -timeout 60s` passed.
- Omron CLI tests passed (`go test ./cmd/naut -run Omron -timeout 60s`).
- `go vet ./omron/... ./tools/omron-research/corpus` passed.
- All 28 generated CXP/CXT case hashes matched their manifests; original
  user-project hashes were unchanged. PowerShell harness parsing passed.

The complete plan is not implemented. In particular there is still no accepted
compiled UM oracle, native production compiler, complete CPU-qualified palette
or enabled deployment path. The next interactive action is to restore desktop
helper access and open the prepared cases offline in CX-Programmer; the exact
initialization failures were `windows sandbox failed: helper_unknown_error:
setup refresh had errors` (node_repl) and `trusted Node process exited
unexpectedly` (CUA). Static research can continue independently using this harness.

## Desktop access recovery

After the user granted their own Windows account WRITE_DAC on E:\cx-proger,
normal sandboxed command startup and node_repl initialization succeeded.
The previous sandbox setup failure is resolved. The restored OSLicence.dll
also allows CX-Programmer to start. Sandboxed writes to an existing design
file still returned access denied; that write required explicit escalation.

Computer Use launched CX-P.exe and read the main window and Open Project
dialog via accessibility. Ctrl+O and Escape were verified by resulting UI
state. Screenshot capture still failed with FrameArrived/window-capture
timeouts; element click reported unavailable coordinate geometry and setting
the filename by accessibility index failed. The dialog was closed without
opening a project. No compilation or PLC connection occurred in this check.

## User-assisted source opening

The user reported opening the requested CJ1M baseline project. The supplied
PLC Properties screenshot identifies NewPLC1 as CJ1M CPU21. Execute
Timer/Counter as Binary is unchecked; Use comment instructions, Use section
markers and Use IR/DRs independently per task are checked. CPU unit version
was not shown. No compile result or executable output was supplied.

Opening also produced failures to create seven controls: Data Trace, PLC
Memory, PLC Settings, IO Table, Error log, PLC Clock and Memory Card OCX.
Consequently, source opening is reported, but full configuration loading and
compiler acceptance are not established. A read-only registry check found
CX-One_V4 4.40.0005 and CX-Server RT 5.0.15.5, but no CX Common Tools entry in
the checked machine uninstall keys. The original Common Tools launcher exists
at E:\cx\CXONE v4.40\CX Common Tools\productsetup.exe, with its Setup.ini and
CX Common Tools.msi. The next manual step is installing/repairing this package
and reporting any installer error before continuing oracle compilation.

## OCX verification after maintenance

The user reported the same seven OCX warnings after the main CX-One wizard
reported Maintenance Complete. Read-only inspection confirmed six specific
PLC Tools controls are absent from their MSI-declared Omron Components
folder and have no 32-bit HKLM InprocServer32 registration: DataTrace.ocx,
PLC Setup.ocx, IO Table.ocx, PLC Error.ocx, PLC Clock.ocx and Memory Card.ocx.
Their CLSIDs and destination were read from the original Common Tools MSI,
not guessed. PLC Memory's concrete server has not yet been identified.
CX Common Tools still had no entry in the checked uninstall keys. NetFx3 is
now installed (3.5.30729.4926), but its installation did not change the failure.

The next bounded manual test is offline PLC > Compile All PLC Programs on the
already opened baseline, with output-window diagnostics retained. This will
determine whether native source compilation can proceed independently of the
missing PLC Tools controls. No compiler acceptance or full configuration
loading is asserted in advance. The GUI component installation remains an
open issue, especially before settings/I/O validation and deployment.

## Automated GUI attempt after user delegated offline compilation

Computer Use launched CX-Programmer, opened MRU item 2 and dismissed the seven
reported OCX error notifications using their OK/default action. A read-only
check of Greg's CX-Programmer v9.6 Recent File List confirmed File2 is the
prepared cj1m-experiments\baseline\project.cxp. The loaded UI identified
NewPLC1[CJ1M] Offline and both task programs. The source CXP hash remained
identical to its corpus manifest.

The PLC menu exposed Compile All PLC Programs (F7). F7 was sent while the
diagram was active, but no compiler result was observed. The Output panel
was then opened through View > Windows > Output and exposed no list items.
With Output focused, Compile All was explicitly disabled. Keyboard navigation
to the project tree did not yield a confirmed selected PLC; later accessibility
captures returned empty state even after reacquiring and activating the window.
Therefore compiler invocation/success is NOT established. No transfer, online
connection, mode change or source save was performed. The project remains open.

Raw accessibility observations were retained locally at
E:\cx-proger\cxp-gui-attempt-20261114.txt. Screenshot capture still timed out.
Next manual diagnostic, once convenient for the user: select NewPLC1 in the
project tree, verify Compile All is enabled, and capture actual output. This
is an automation/focus limitation, not evidence of a compiler failure.

## First user-confirmed CX-Programmer source check

The supplied output log accepted the prepared baseline for CJ1M CPU21 with
0 errors and 79 warnings: 78 duplicated-output diagnostics and one unused FB
instance BLINK_L2. Both MotorDrive_LIFT_1 and NewProgram1, plus the BLINK
function-block definition, appear in the check log. The selected program-check
option is Unit Ver.4.0; this is not a measurement of a physical CPU version.

The baseline CXP still hashes to
07b842a8a2a9830f4778ee1ef4ed07111b5b508328adb1fec9055c388903a7c7.
The supplied log SHA-256 is
34e054c93acbd7fd7333347c32a0d2d44c226a185f99eb158b93b89c0e461a40.
The exact log and source-linked result.json are preserved under
research/cj1-oracle/cj1m-experiments/baseline/oracle (outside the repository).

This establishes CX-Programmer source-check acceptance for this case despite
the missing PLC Tools controls. It does not establish native executable output,
serialized UM correctness, settings/I/O loading, hardware execution or upload.
Next: obtain the analogous CJ1H check and capture actual compiler binary output
through a confirmed export path or the compiler's in-process output boundary.
The existing duplicate-output warnings are retained; no control logic was
rewritten to suppress them.

## CJ1H baseline source check confirmed

The user-supplied CX-Programmer log also accepts CJ1H-H CPU65: 0 errors,
79 warnings (78 duplicated outputs and unused FB instance BLINK_L2).
Both task programs and BLINK are present; the program-check option is Unit
Ver.4.0, not an observed hardware firmware version.

Source SHA-256: 1fe7d850676d7d0fe19510ceb58c7fe9ab2afd9bee1510f22a5bb8047abbd677.
Log SHA-256: c12eaa22933cba0eb3609eb5e8409b4c2faf2aabedaefc9d69fa3077d73e3903.
The exact log and result.json are preserved under
research/cj1-oracle/cj1h-experiments/baseline/oracle.

Both baselines now have source-check acceptance. The generated corpus
manifest remains the original source-generation snapshot; individual oracle
sidecars record subsequent results. Executable UM capture and comparison,
settings/I/O validation, and hardware deployment remain unverified. Next work
is to capture compiler binary output for controlled differential experiments.
