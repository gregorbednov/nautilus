# Omron CJ1M / CJ1H backend â€” implementation in progress

## Create and build a CJ1 project

In Nautilus VS Code, choose **Create Project… → Omron CJ1**, select the CPU
and timer mode, then create an empty task or import a CXP/CXT and its N1
settings JSON. Open the new folder. **Build Omron CJ1 Project** saves edits to
`cj1.json`, `program.cxt` and `settings.json` in the selected folder and
displays the CLI build report in the Nautilus CJ1 output channel. For an
`st-il` project it saves `cj1.json`, `settings.json`, `program.json` and
the ST files named by the current definition list before running the build.
The desktop extension and CLI both need a build containing these commands.

```sh
naut omron new-project -model "CJ1M CPU21" -timer-mode bcd lift
naut omron build-project lift
```

Use `-source FILE -settings SETTINGS.json` before the new directory name
to import an existing project. The project keeps its selected CPU, timer mode
and resource in `cj1.json`, editable ladder source in `program.cxt`, and image
allocation settings in `settings.json`. Edit the CXT as text. The empty starter
contains one task with END. Import writes the editable CXT as UTF-8. The
reader accepts UTF-8 and uses Windows-1251 for legacy files in auto mode.
Review source settings and allocations for the intended installation.

For a graphical view, open `program.cxt` with **Ladder Diagram (Omron CXT)**
in VS Code. The read-only view displays cyclic tasks and their sections in
source order. Unsupported rungs appear as source text with the reason they
could not be drawn. The same model is available from `naut omron graph FILE`.

Choose **Watch CJ1** in that diagram to connect to a FINS/TCP endpoint. The
diagram shows sampled contact, coil, and timer values. **Variables** lists
each symbol's scope, address, type, value, and quality. A task's declaration
takes precedence over a resource global of the same name. The status shows
the controller's current mode. Values are samples of CPU memory, independent
of which program is installed. Readout stops when the diagram closes or its
source changes.

Builds use the same independent compiler as `compile-image`. Each build
publishes `build/<sha256>.n1`; an identical build verifies the existing file.
A conflicting file at that name is reported as an error. Both commands work
offline. The compiler report gives the scope of the current qualification.

### Build the FB source archive

`build-fbl` generates a local `SRCPRGRM.FBL` from a CXP or CXT project with
one ladder FB definition, one or two saved instances, and the observed 1.8
source layout. The archive carries the FB source, its symbols and allocations.

```sh
naut omron build-fbl -o SRCPRGRM.FBL project.cxp
```

The command preserves the definition's `Modified` timestamp by default.
Use `-modified 'D M YYYY H M S'` with the timestamp from a CX export when
reproducing its bytes. The CJ1M new4 archive matches the 1117 bytes in the
captured CX transfer with `-modified '23 11 2026 2 2 13'`.

### Download a CXP or CXT program

`Download Omron CJ1 Project` in desktop VS Code saves the open CXT project
inputs, reads the chosen CPU over FINS/TCP, and shows a confirmation before
writing. Select **Download and leave in PROGRAM** or **Test RUN and restore
original**. For the temporary run, select the exact original UM and optionally
the number of consecutive task flags to verify. For the verified new4 project
select its resident UM as the readback reference. The **Nautilus CJ1** output
channel shows the result and the path to the journal in
`build/download-*/journal`. The same run directory holds the input snapshot
and the CLI result.

`download` uses direct FINS/TCP and starts from a controller in PROGRAM.
Capture the intended CPU identity with `naut omron status -transport tcp
-endpoint IP:PORT` and pass its JSON output as the target. The command builds
the program and any required FB archive together. It saves the original program
pair in a new journal directory before writing.

```sh
naut omron download -model "CJ1M CPU21" -timer-mode bcd -settings settings.json \
  -target status.json -endpoint IP:PORT -journal new-journal -execute project.cxp
```

Pass `-project DIRECTORY` (with the directory as the last argument) to
use `cj1.json`, `program.cxt` and `settings.json` in place of the standalone
compiler arguments. The project compatibility checks also apply to download.

For a supported CJ1M CPU21 FB layout, the session verifies the program and
reads back every byte of `SRCPRGRM.FBL` before it can request RUN. If the
original already has an archive, the session saves two matching reads in the
journal as one backup and restores those bytes with the original program.
If either archive write receives a FINS `2108` refusal during a replacement,
the session verifies the program and files in PROGRAM before repeating the
same write once.
Use `-resident-reference` with the retained new4 UM when checking the verified
new4 image whose service words are finalized by the CPU.

For the verified CJ1H-H CPU65 new4 build, the downloader generates the
Programs-only FB archive layout and requires the corresponding saved
resident image. The bench check covered the compiled image, archive
readback, RUN, VS Code ladder and variable values, and byte-exact return
of the original program and catalog.

Multiple read-only FINS/TCP watchers can poll the same CPU concurrently.
Run program transfers one at a time. The downloader requests programming
access from the CPU and stops on a busy response without forcing access.

Add `-run` to request RUN after verification. For a temporary bench run,
add `-return-original -expected-original baseline.um`. The command returns
the pinned original to PROGRAM and verifies it over two fresh connections.
`-check-task-flags 2` checks TK0 and TK1 during RUN. The journal and return
cover the program and its FB archive catalog. PLC settings and data memory
require separate verification.

For the reconstructed CPU21 temperature/humidity application, run
`naut omron build-project omron/internal/research/cpu21sample` from the
repository root. Edit the two ST files for the arithmetic and `program.json`
for the task instructions, instance-to-definition bindings and H addresses.
The project's `sourceFormat` is `st-il`. Task instruction `CALL 6 2097 D100`
uses instance 6, reads CIO word 2097 and returns a two-word result to D100.
The sample builds offline with explicit instance placement and a BCD profile.

`naut omron inspect program.cxt` reports `minimumCJ1UnitVersion` on textual
STR4/8/16 and NUM4/8/16 instructions. These require unit version 4.0 on
CJ1M CPU21 and CJ1H-H CPU65. Building a project with one currently produces
a source diagnostic. [Native version checks](../docs/design/omron-cj1-conversion-versions.md)

For function block headers, the same command reports source paths and lines in
`fbDeclarationFindings`. An In Out BOOL or UINT requires CJ1 unit version 4.0.
STRING variables require a Structured Text function block and Internal or In Out
usage. [Declaration checks on both CPU profiles](../docs/design/omron-cj1-fb-declaration-versions.md)

The [source FB invocation comparison](../docs/design/omron-cj1-source-invocation.md)
now reproduces four complete 3180-byte native images with explicit current/check
versions 3.0/4.0 on both profiles. Public CompileCXPImage and CLI match the full
files from a retained CXT, while four missing-LD controls fail native checking.
This bounded file comparison does not qualify runtime execution or lift the
explicit UnitVersion gate. No PLC traffic was used.

Offline [current3/current4 comparison](../docs/design/omron-cj1-version3-compile.md)
now reproduces 5560 instruction results and 16 complete native N1 files.
`compile-image -timer-mode binary` supports audited textual resources without FB;
BCD FB support remains bounded by its existing qualification. Explicit codec
UnitVersion3.0 and hardware write/restore qualification remain closed.
Go replay uses only retained fixtures, without CX-One, Windows or a PLC.


Research now traces a concrete manufacturing-information candidate command0226
and model-specific service declarations. Selector meanings and response unit
version decoding remain unresolved; no new network API or PLC request. See
[manufacturing route](../docs/design/omron-cj1-manufacturing-route.md).

The settings audit now reports retained local library state separately:
CJ1M/CJ1H default/current bytes[4,0,0], IsExtendedProtect0 after SetPLC.
These observations do not select hardware unit version or transfer service.
See [local profile evidence](../docs/design/omron-cj1-local-profile-state.md).

Read-only `ReadConnections` now enumerates FINS0502 CPU/CPU Bus/Special I/O
model records with bounded pagination and strict responses. Two live lists
match on each CPU: H has ETN21/DRM21/MAD42(unit3), M has
ETN21/SCU41-V1/MAD42(unit9). Both remained RUN/error0; raw streams verified.
See [connection data](../docs/design/omron-cj1-connections-2026-11-19.md).

Both real CPUs now returned two identical complete copies of all four
parameter areas: PLC Setup, registered I/O, routing and CPU Bus Setup. Raw TCP
reassembly verified16 files/59904 bytes and read-only command scope; both
CPUs remained in RUN with no errors. See
[actual parameter captures](../docs/design/omron-cj1-parameter-pairs-2026-11-19.md).

Read-only `ReadParameterArea` now implements complete selected PLC Setup, I/O
registration, routing and CPU Bus parameter reads with strict0201 validation.
Local-only verification; no full-backup or restore claim. See
[parameter read preparation](../docs/design/omron-cj1-parameter-read.md).

Mode/access control has now been observed on both real, operator-isolated CPUs:
CJ1H/.37 and explicitly approved CJ1M/.36 each completed RUN->PROGRAM->RUN,
release ACK and fresh read-only UM verification without a program write.
CJ1H retained2716 bytes; CJ1M already held4672 bytes before its trial and retained
them exactly. Full program transfer/restore qualification is still open.
See [CJ1M follow-up](../docs/design/omron-cj1m-mode-only-2026-11-19.md).

`NewControlledProgramSession` now saves backup before journaled acquire/STOP,
requires PROGRAM readback before transfer, and exposes explicit execution and
finish/release. Fresh original-return sessions acquire access again; failures
never force/retry/release/RUN automatically. Both profiles pass local tests.
An actual CJ1H/.37 mode-only cycle RUN->PROGRAM->RUN succeeded with unchanged
2716-byte UM and release ACK, verified through a fresh read-only connection.
No program write occurred; full download/restore remains unqualified.
See [controlled session](../docs/design/omron-cj1-controlled-session.md).

`fins.Client` now provides explicit `RequestCPUMode`, `AcquireProgramAccess`
and `ReleaseProgramAccess` wire APIs with strict ACK and terminal errors.
Release is global, so Close/error paths never release automatically. Busy3001
retains the reported owner. Local TCP tests pass on both profiles; lifecycle
integration with the program journal remains open. Real devices were read only:
CJ1H2716 bytes and CJ1M4652 bytes, consistent pairs with stable CPU/status in RUN.
See [mode/access and fresh backup results](../docs/design/omron-cj1-mode-access.md).

`commission.ProgramSession` now owns a dedicated TCP connection, saves a fresh
PROGRAM-mode backup pair and synchronous journal, writes the candidate and
requires exact readback. A separate one-shot call can return the private original
on a newly supplied connection. Failure never retries/resumes or automatically
returns/RUNs. Tested on loopback only; mode/access lifecycle, full backup and
hardware qualification remain open for this original API path.
See [program session](../docs/design/omron-cj1-program-session.md).

`fins.Client.WriteProgramBlock` is now an explicit mutating Go API with exact
0307 ACK validation and terminal session failure, without retries or mode changes.
The download command uses this sender. See
[block sender](../docs/design/omron-cj1-0307-block-sender.md).

`audit-program-backup -model MODEL -before JSON -after JSON UM1 UM2` now checks
retained identity/status consistency and exact program copies, including service
words. RUN is allowed for backup review; success is not full-backup/restore or
write approval. See [backup review](../docs/design/omron-cj1-program-backup-review.md).

[Profile/version origins](../docs/design/omron-cj1-profile-version-origins.md)
separate archived worker masks from saved CS2SupportVersion and live unit version.
The audit reports evidence scope and keeps unit/profile qualification false.

[BR2 and FB profile](../docs/design/omron-cj1-br2-and-fb-profile.md) audit keeps
saved BR2 separate from the pinned DIFF8191,0 default candidate. Source UseFB
cannot prove the active support/version branch; profile qualification stays false.

[Stack selection](../docs/design/omron-cj1-stack-source-origin.md) now distinguishes
missing/true UseDefault from the explicit branch. Saved D0 is inactive in the
four projects; the pinned default table is AC0/64/1536. Auditing this does not
qualify the actual CPU profile or generate initial data.

`audit-image-settings -model MODEL -resource PATH -settings JSON FILE` compares
saved FB-memory text with the recipe and inventories declaration column5.
Missing BR2 and D0/AC0 differences remain explicit; no initialization is emitted.
See [source settings audit](../docs/design/omron-cj1-source-settings-audit.md).

Optional preflight `-source FILE -settings JSON -resource PATH -timer-mode bcd`
now recompiles the exact recipe and requires byte-for-byte N1 equality. The
in-process proof is bound to model and image; report JSON cannot grant approval.
See [recipe reproduction](../docs/design/omron-cj1-build-reproduction.md).

`preflight-program-write -model MODEL -snapshot STATUS.json -backup UM1 -backup-check UM2 IMAGE`
now assesses retained raw CPU model/status, reported capacity and two program
copies. Conflicting summaries cannot override raw data. Freshness, unit version,
source-settings agreement and full backup stay unverified; no network or write approval.
See [offline preflight](../docs/design/omron-cj1-offline-preflight.md).

`audit-program-write [-readback FILE] IMAGE TRACE.json` now audits retained
attempts/ACKs offline and compares retained readback. Failure stops progress;
service-only changes are reported, not accepted. Diagnostic inspection after
failure cannot resume the audit. See [offline audit](../docs/design/omron-cj1-offline-write-audit.md).

[Finalization boundary probes](../docs/design/omron-um-finalization-boundaries.md)
now compile exactly against native files on both CPU profiles (2632/2824 bytes).
Service diagnostics expose all four studied word4 hypotheses; none is selected
as the target algorithm. Existing readbacks do not distinguish the24-byte prefix.

The [native file-to-block path](../docs/design/omron-cj1-file-byte-path.md) now
connects the selected compiler filename to whole-file copying and0307 data bytes.
Raw N1 planning rejects OBJ input unless explicitly unwrapped. Target/profile
selection and finalization remain unqualified; no send API was added.

Current readiness and prioritized acceptance gates:
[2026-11-18 review](../docs/design/omron-cj1-readiness-2026-11-18.md).
Minimal native standalone task files now match independent extraction on both
CPU profiles (four pairs). The absent-task native control returns success with
an empty file; portable extraction rejects the missing task. Full-image wire
mapping, CPU finalization and hardware writes remain unqualified.


Hardware access and migration procedure:
[verification and handoff plan](../docs/design/omron-hardware-verification-and-handoff.md).
Latest operator report (18 November): CJ1H/.37 and CJ1M/.36 are online and
disconnected from actuators except LEDs and other insignificant loads. Both
can enter read-only preparation. Mode/access, protection and recovery checks
still precede any hardware write; qualification remains false.

The local CJ1 object envelope is now traced:48 zero bytes followed by the complete
N1 body when protection is disabled. `um.WrapPlainN1Object` and
`um.ParsePlainN1Object` support only this plain subset; protected/CJ2/metadata
variants fail closed. These are local file APIs, not wire payloads or a writer
for the PLC; CLI integration and a native OBJ golden sample remain open.
See [plain object envelope](../docs/design/omron-plain-object-envelope.md).

The observed CX-Programmer branch uses `PLC_KBase.IsExtendedProtect` to choose
standard service3 or extended service532. A local file-header path assigns both
0 and 1; further paths parse 380-byte blocks from a file body or trailer.
This file header is not yet mapped to raw N1, and the live protection state
remains unmeasured. The second setter's object is confirmed on its cached-KBase
branch; its fallback pointer still needs typing. Static verification: 79 checks.
See [dispatch](../docs/design/omron-extended-protection-dispatch.md) and
[file origin](../docs/design/omron-extended-protection-origin.md). The following file operation reaches
`CEncryption::CompositeUMObject`; it is conditional and does not establish a
raw N1 transform. 64 further static checks passed. See
[OBJ workflow](../docs/design/omron-obj-protection-workflow.md).
The N1 archive writer and separate EncryptUMObject path are now traced;
a conditional post-write conversion is CJ-to-ULC, not a general OBJ wrapper.
55 static/replay checks passed. See
[N1 writer](../docs/design/omron-n1-writer-and-protected-file.md).
Caller12612097 is now located in a task helper reached from the dispatcher;
its services463/526 differ from full-program3/532. The XML inspector supports
`--family task`; all eight default program reports remain unchanged.
See [task/program dispatch](../docs/design/omron-task-versus-program-dispatch.md).

Portable research tool `tools/omron-research/inspect_cj1_services.py` now reads
the installed or copied StaticRoot XML without COM/vendor DLLs. Both project
profiles expose separate services3 and532, not interchangeable versions of one
operation. Eight profile replays matched a retained21-file XML snapshot; ten
synthetic tests passed. See [CJ1 service XML](../docs/design/omron-cj1-service-xml.md).

Native `ProgramDownloadEx` uses command0317, while the standard service uses0307.
The offline0307 plan now explicitly leaves this target/service-version choice
unqualified, and its ACK validator rejects0317 responses. The first transaction
also performs status handling and reads the source file; full header preparation
remains open. See [service versions](../docs/design/omron-program-service-versions.md).

`transfer.ValidateProgramWriteResponse` validates retained0307 request/response
bodies and accepts only an exact full-block ACK. It returns no acknowledged
progress on errors and performs no networking or retries. Native CS1 transaction
code corroborates the candidate block format; CPU/service-version selection and
full image mapping remain unqualified. See [PnP and program transfer](../docs/design/omron-cj1-pnp-program-transfer.md).

The native sender path is now traced from CX-Programmer through
`CDMDownloadProgram` to the CX-Server communications handler. This establishes
a file-name request boundary, not the FINS wire mapping. The Ghidra exporter
now includes decompiler-discovered blocks in its assembly evidence; 23 static
checks passed. See [sender-path research](../docs/design/omron-cj1-native-transfer-path.md).

Native follow-up: isolated100-byte arithmetic code passed80 cases; four real UM
images also match word4=ceil(bytes/64). A local empirical-header copy of the
unchanged4604-byte CJ1H CXP build now matches its historical readback entirely.
The virtual CPU finalizes headers on the last0307 block, but its resident256-byte
quantum is still unmapped to the real64-byte relation. No stamping/sending API
was added; full target finalization remains unqualified. See the
[native analysis](../docs/design/omron-um-native-sums-and-finalization.md).

`plan-program-write [-block-bytes 996] FILE` now prints a local candidate0307
block plan with image/block hashes and exact payloads. There is no network or
execution option. All bytes are preserved; deployment remains unqualified.
`N1Image.ServiceFields()` reports an observed relation matching four live UM
captures: words0/1 split sum32(words6..end). Word4 and native stamping rules
remain open; no header-writing API was added. See the
[service fields and offline plan](../docs/design/omron-um-service-fields-and-write-plan.md).

`inspect-image -decode-tasks -model MODEL -timer-mode bcd FILE` now decodes
qualified task instructions, verifies exact re-encoding and preserves an opaque
tail from the first unsupported form. It reports code/image offsets and bounded
coverage, not restored symbolic source or graphical LD. All24 code words of the
stable CJ1H counter example decode to18 instructions and recompile exactly through
a derived CXT. See [task-code readback](../docs/design/omron-cj1h-task-decoding-2026-11-16.md).

Latest CJ1H snapshot after an intervening class is again different:2716bytes,
no FB, one task. At that capture .36 was offline; the latest operator report
above supersedes that restriction. On `.37`,0502 reported
ETN21/DRM21/MAD42; the complete8012 registration table matches both new4 and
original CXP. This confirms registered configuration, not the full physical
Basic I/O rack or symbol names. Symbols remain project declarations. See the
[latest inventory report](../docs/design/omron-cj1h-new4-inventory-2026-11-16.md).

After the operator's manual CJ1H download, two new4604-byte UM reads match
our unchanged-CXP build in every word except three unknown service words at
0/1/4. All task/FB code and records, configuration and directory bytes match.
This qualifies this source-to-readback comparison, not agent deployment or
execution. The operator's "failed to download data" warning with all segments
selected remains unresolved; other download segments were not verified.
See the [CJ1H readback report](../docs/design/omron-cj1h-manual-download-readback-2026-11-16.md).

Complete offline linking also supports resources with no FB: use an explicit
empty `definitionNumbers` map and zero next-enable state. The2624-byte minimal
CXT build matches both archived native CPU files exactly. The linker rejects
unmatched global/local FB symbols and saved instances; it emits no empty FB
headers. Other task/instruction/settings limits still apply. The initial live
UMs differed structurally from both source projects in `projects`; see the
[source comparison](../docs/design/omron-hardware-source-comparison-2026-11-16.md).

The working pieces are independent of Windows, Wine, CX-One and vendor DLLs:

`naut omron compile-image -model MODEL -resource PATH -timer-mode bcd
-settings SETTINGS.json -o NEW.n1 FILE` now builds the complete qualified N1
from CXP/CXT and explicit options/memory/IDs. Both native4592-byte fixtures match
exactly for their source recipes; unchanged sources give4604 bytes. Existing
outputs are preserved. Instance containment and internal references are checked;
storage initialization, source setting provenance and hardware remain open.
See [usage and current scope](../docs/design/omron-n1-build.md).

`codec.EncodeN1Header` now generates all42 fixed header words from explicit
options, configured memory and section lengths. It matches both saved project
headers. Twelve new complete files with a simple FB definition qualify changed
FB/stack memory emission; every689-word file matches independent Go output.
The resource-body command still returns separate components; compile-image now
links them for the qualified scope. Source option and storage/hardware work remains.

`codec.EncodeN1FBMemory` now generates the configured17-word FB/stack descriptor
from explicit settings. It matches controlled native getters and the saved FB
header's default values. The native writer emits it only when definitions exist;
12 complete files without definitions correctly retain zero memory header fields.
CXP stack provenance and wider FB layouts remain open;
no default stack address is silently substituted for saved D0.

`codec.EncodeN1TaskFlags` now generates five header option bits from explicit
IR/DR, timer mode, Extended TIM Hold and copy/overwrite settings. All64 native
CPU/mode/option combinations match in portable Go replay. This API covers ordinary
unprotected tasks; CXP option provenance and extended header modes remain
open. Native defaults are not silently substituted for project settings.

`compile-task-configuration -model MODEL -resource PATH -timer-mode bcd FILE`
generates all288 header task-configuration words, matching both native fixtures.
Task startup now supports OpStart0/1; protected tasks remain rejected. The
resource-body command also reports this table separately, without changing its
post-header words. Compile-image integrates the header; password handling remains pending.

`compile-task-block` generates the288-word task directory and sorted records;
all652 words match both native fixtures with their captured source recipes.
`compile-resource-body -model MODEL -resource PATH -timer-mode bcd -number BLINK=0
-next-enable 0,0,0,0,0,0,0 FILE` combines this with both FB sections and checks
FB call links. All818 post-header words match both native images. Unchanged
original source gives821 words. These fragments have no global N1 header or
initial storage; task configuration is returned separately by the resource
command. Compile-image performs final linking; hardware remains pending.

`naut omron compile-task-record -model MODEL -program PATH -timer-mode bcd FILE`
generates a standalone task record, including its32-word header and entry/return.
All188/176 words match both native tasks on both CPU fixtures with the captured
source recipes. Scope: IDs256..287, OpStart0/1, unprotected, ASCII identifier names
of1..24 bytes, qualified mixed body. Original unchanged first program gives191
record words. The standalone record excludes task configuration/directory and
global header; the configuration table is available separately as above.

`compile-program-body -fb-calls -timer-mode bcd -section-markers` now composes
the observed scalar FB invocation with all textual rungs in one program.
With first-rung source recipes reconstructed and hash-checked from the oracle
manifest, all151 first-task code words match both native captures. Unchanged
original CXP gives154 words due to its different first rung. At most one FB call
is qualified per program; IL must close before calls and section boundaries.
Task headers/wrappers, FB records and complete images remain separate.

`naut omron compile-fb-definition-section -model MODEL -resource PATH -timer-mode
bcd -number BLINK=0 FILE` generates the complete observed definition section,
matching all50 words on both CPU fixtures. Repeat `-number NAME=N` for every
definition; record IDs remain explicit and independent of body TaskIDs.
`compile-fb-prefix` adds the audited instance section with required
`-next-enable 0,0,0,0,0,0,0`: all166 FB-fragment words match both native images.
It checks internal instance-to-definition names/paths, not all callers or tasks.
There is no N1 file header, task directory/code, initial storage or full image.

`naut omron compile-fb-instance-section -model MODEL -resource PATH -timer-mode bcd
-next-enable 0,0,0,0,0,0,0 FILE` generates the observed full instance section;
all116 words match both saved native fixtures. The seven header fields are
SetNextEnableAddress state, separate from instance footprints. Only an explicit
zero state is qualified. No initial storage, inferred next-enable values or
complete image linking is generated; see the FB format document for evidence.

`naut omron compile-fb-instances -model MODEL -resource PATH -timer-mode bcd FILE`
compiles all saved global FB instance records after checking their global symbol
identities and pairwise T/H memory overlap. Local instances and local names
shadowing these globals fail. The report exposes absolute half-open word ranges.
It does not audit scalar symbol/operand memory, all callers or task links, and
does not generate a section header, initial storage or new placement.

`naut omron compile-fb-instance -model MODEL -instance PATH -timer-mode bcd FILE`
now generates the observed scalar FB runtime frame and ten-word instance header
from CXP, using saved placement. All54 words match both instances on both CPU
fixtures. Scope: word,word,BOOL inputs and BOOL output plus EN/ENO. Storage
initialization, global placement audit, section/task linking and the complete
N1 image remain pending; see `docs/design/omron-fb-format.md` for evidence limits.

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
  short typed BR0/BR3 encoding is also available through
  `codec.EncodeFBRelativeInstruction/Body`, matching218 native compilations.
  It reconstructs the19-word saved bodies with already allocated references;
  a bounded offline scalar allocator now connects it to CXP definition source.
  `compile-fb-definition-body` produces all19 body words and15 source spans
  from both original projects, with exact N1 agreement and40 native allocation
  cases. `compile-fb-definition -number N` also generates the entry/return and
  record header: all46 words match each native BLINK definition record.
  Number assignment is explicit. Instance placement/initialization, section
  headers and image linking remain pending.
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

[ÐŸÐ»Ð°Ð½ Ñ€ÐµÐ²Ð¸Ð·Ð¸Ð¸, Ð´Ð°Ð»ÑŒÐ½ÐµÐ¹ÑˆÐµÐ¹ Ñ€ÐµÐ°Ð»Ð¸Ð·Ð°Ñ†Ð¸Ð¸ Ð¸ Ð²ÐµÑ€Ð¸Ñ„Ð¸ÐºÐ°Ñ†Ð¸Ð¸](../docs/design/omron-revision-and-delivery-plan.md)
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
present values. `TK0` through `TK31` read CJ1 cyclic task flags as one byte
per task. Use `TK0 32` to read all flags, or bind a single flag as a BOOL
in the monitor. Wire-family range validation is not a CPU capability check.

On 2026-11-22 the Windows CLI read all 32 flags from both bench CPUs
(CJ1M-CPU21 and CJ1H-CPU65H) over FINS/TCP using
`naut omron read -transport tcp -endpoint HOST:9600 TK0 32`. Both
returned 01 for TK0 and 00 for TK1 through TK31. These readings describe
the programs present during the test.

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
