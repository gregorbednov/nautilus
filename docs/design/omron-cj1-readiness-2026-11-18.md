# CJ1M/CJ1H readiness and next acceptance gates — 2026-11-18

## 20 November 2026 program backup check

Two independent UM reads per controller matched each other and the originals
from the respective 19 November mode-only sessions. Both CPUs reported RUN
and zero fatal/nonfatal status before and after the reads.

| CPU | UM bytes | SHA-256 |
| --- | ---: | --- |
| CJ1H-H CPU65 | 2716 | d54d9fcb0e4c5f6fb95bc16c31b07fd55a880982835d31b6676af744a1c34a46 |
| CJ1M CPU21 | 4672 | a4c0b45ba391fcd0cefb49bce52058168971e4aca1e6b29fe05937f80961c8cf |

Offline preflight against each model's 4984-byte BOOL control passed the
source recompilation, backup-pair and reported capacity checks. Both reports
have `readyForWrite=false`. Hardware recovery, finalization, initial data and
project/device settings still need verification. The 19 November parameter
archive passed its 49-file manifest check. The 20 November capture refreshed
only program memory and status. Raw observations and preflight JSON are in
`research/hardware/20261120-cj1-backup-control` in the enclosing workspace.

### Parameter comparison after the program backup

On 20 November both CPUs also returned two complete copies each of PLC setup
(1024 bytes), I/O table (2560), routing table (1024) and CPU bus setup (10368).
For each CPU and area all four files from the two dates matched byte for byte.
The wire replay reconstructed 16 copies from 72 parameter-read responses.
The capture also contained 16 identity and 16 status requests. Both CPUs
reported RUN and zero fatal/nonfatal status throughout the observations.

The recovery procedure can use the saved bytes as a comparison baseline
before and after a program-only trial. These observations cover eight named
CPU/area pairs. Retained application data, initial values and CPU finalization
still require separate evidence. A successful program readback alone cannot
establish restoration of those values. The streams, copy files, verification
and `archive-comparison.json` are in
`research/hardware/20261120-cj1-parameter-pairs-01`.

The offline command `naut omron compare-parameter-backups -model MODEL BASELINE_DIR OBSERVED_DIR`
rechecks both copies of each area against their recorded sizes and hashes and
compares the recorded endpoint and raw CPU identity across all eight pairs.
It reports the changed-byte count and first differing byte per area. Invalid
archives and differences both return a failure status. The two November
captures matched for all 14,976 parameter bytes per CPU.

The portable CPU21 archive is
`research/hardware/20261120-parameter-review-131545/cj1m-recovery-evidence.zip`
(SHA-256 `bdb34ba81313b085241ab2777b4b73d936648afca04eae1ffc8b025b8672b0f8`).
It includes the exact UM pair, parameter pairs and streams, timestamped
application-state samples and the editable ST/IL project. After extracting it
into a new directory, both the parameter comparison and source rebuild were
repeated. The rebuilt body matched the saved UM, with differences confined to
service words 0, 1 and 4. This portable check exercises file recovery only;
physical restoration and retention of runtime state still need evidence.

### Application state and exact readback

The 2716-byte CPU65 backup contains one task (256, 24 decoded words) and no FB
definitions. Its two CNT instructions use C120/C121. Two read-only samples in
RUN gave present values 0/3 and completion flags 1/0. W0 was 0004, CIO0 was
0000 and CIO4 was 5400 (hex words). The enclosing UM reads matched the backup.
These are sequential observations. A recovery check must account for counter
state and externally driven I/O. The CIO samples are observations only.

The CPU21 backup contains Humidity and Temperature definitions and six
instances. Task 256 stopped decoding at word zero of 53; the relative-body
decoder also rejected a definition. Its application data footprint remains
unresolved. The 4984-byte test project's own saved allocations include
H512..H518 and T3072..T3075; its direct timers include T0..T6.

`naut omron compare-readback EXPECTED.n1 OBSERVED.um` compares local files
and reports the first differing byte, service-word flags and body equality.
It succeeds only on exact equality. The historical 4604-byte CPU65 readback
after manual CX-One download differs only in zero-based words 0, 1 and 4.
This is a diagnostic case; the CPU transformation remains unqualified.
For restoration compare against the exact pre-attempt UM from that CPU.
The observations and replay reports are in
`research/hardware/20261120-cj1-state-inventory-01`.

### CPU21 task 256 follow-up

The pinned CX-Pdata offline decoder identified `e212400f` as `LD CF113`
at task word offsets 0, 22 and 44. A separate native instruction build in
BCD at local current version 3.0 emitted the same word on CPU21 and CPU65.
The Go decoder now reconstructs both measured CIO-input / DM-result call
frames and verifies the complete task body (25 instructions, 53 words).
Every instruction boundary, operand and word matched the native disassembly
(the public codec spells END(001) as END). Source names and a complete
independent rebuild of this running program remain open.

The native disassembly covers all 53 task words, both FB definition bodies
(52 and 62 words) and all six instance code sections (38 words each).
Each returned sequence was compared to the corresponding slice of the
4672-byte UM backup. Task 256 calls instance 6 / Temperature (body 1024)
with CIO2097, and instance 5 / Humidity (body 1025) with CIO2098.
The MOVL outputs occupy D100..101 and D200..201. The caller also references
W0.02, W0.03, CIO2090.00 and a word transfer from D300 to CIO2091.
The two called instances use BR3 bases H544 and H580 respectively, with
accesses through word 7. Other saved instances use H536, H512, H520 and
H528. In particular the candidate's H512..518 overlaps an existing instance.
These are address findings from captured code. Runtime values, retention
through mode transitions and the peripheral I/O state require separate checks.
Raw evidence lives in the same inventory directory under `task-native`,
`cf113-native`, `definitions-native` and `instances-native`.

Two sequential CPU21 reads on 20 November (08:28 UTC) sampled H512..551,
H580..587, D100..101, D200..201, D300, W0, CIO2090..2091 and CIO2097..2098.
The CPU reported RUN and zero fatal/nonfatal status at both boundaries; both
UM backups equalled the 4672-byte original above. H512..543 were zero in both
samples. The called instances' H ranges, D200..201 and CIO2097..2098 changed
between samples. W0 was 000C and D300 and CIO2091 were both 0E10 in both
passes. These reads establish live values, not retention or restoration
semantics. The timestamped samples and enclosing backups are in
`research/hardware/20261120-cj1m-state-112807`.

The portable `inspect-image -decode-fb-sections` path now verifies 114
definition words (Humidity 52, Temperature 62) and 228 instance words
(six sections of 38). All 120 FB instruction texts, boundaries and rebuilt
words matched the two native decoder logs. Together with task 256 this
covers 395 code words in 145 instructions for the archived CPU21 program.
Run with `-decode-tasks -decode-fb-sections -model "CJ1M CPU21" -timer-mode bcd`.
The full offline report is `cj1m-full-code.json` in the state inventory.
The decoded offsets and literals do not recover original ST expressions or
establish independent generation of the complete 4672-byte image.

An offline reconstruction of this specific application is now reproducible
with `go run ./omron/internal/research/cpu21sample NEW.n1`. Its inputs are
the recovered arithmetic operations, call sites, explicit H allocations, task
name and observed record metadata encoded in the recipe. The builder reads no
CXP, UM, native logs or binary templates. Its 4672-byte candidate hashes to
`3b70cab1f76a274072b0b82cc88dfa9cda37ffdd39fdc3ff4855b1b8794aa6d7`.
Against the archived UM, all words except 0, 1 and 4 match. Those three
are zero in the candidate and 0000A00A, 00003F08 and 00000049 on the CPU.
The regression test now computes the sum and the 64-byte count from the
compiler candidate alone. Substituting those predicted values into a test
copy reproduces the archived SHA-256. The two 64-byte prefix hypotheses
coincide at this file length. See the [six-image comparison](omron-um-finalization-boundaries.md).
This is a reconstruction from measured metadata and selected placements.
Compiling equivalent user-authored FB source, choosing new placements, and
deriving CPU service words remain separate verification steps.

The reconstructed Temperature and Humidity definitions now live in editable
ST files in `omron/internal/research/cpu21sample`. The sample linker takes
their records from the same ST compiler used by `naut omron compile-fb-definition`.
For example, pass `-model "CJ1M CPU21" -timer-mode bcd -number 0 -body-number 1024 Temperature.st`;
Humidity uses 1 and 1025. The CPU21 backend supports the observed one-INT-input,
one-REAL-output, two-local-REAL frame, INT_TO_REAL and single-temporary REAL
arithmetic with a literal operand. Both records, including headers (85 and
75 words), match the archived UM. The full candidate retains the hash above.
The standalone research command retains the fixed reconstruction as a reference.

The same directory is also an editable Nautilus project. Run
`naut omron build-project omron/internal/research/cpu21sample` from the
repository root. Its `cj1.json` selects the CPU21 `st-il` source format.
`program.json` declares the ordered definitions and body IDs, six instances
and their explicit eight-word H ranges, and the cyclic task. Each definition
is compiled from its named ST file. Within the task, `CALL 6 2097 D100`
between LD and OUT expands to the generated FB invocation for instance 6,
with CIO word 2097 as the input and D100..101 as the result. Definition IDs
and bank settings come from `settings.json`. The build resolves calls against
the declared instances and checks for overlapping frames before publishing.
This path reproduces the same 4672-byte candidate and hash above. Changing
the first destination to D101 changes only the corresponding operand byte.
The evidence covers one cyclic task, explicit placement and the observed
word-input/REAL-output blocks on CPU21. Hardware finalization remains open.

On 20 November, CX-Programmer uploaded the CPU21 program in RUN. The saved
CXP (SHA-256 `6328c941cb5061d5b14ab18111ea9e7b875b75b334b002fcc655df8dc05eacd8`)
contains FBST 1.8 declarations and expressions for both blocks.
`compile-fb-definition` can compile these records directly from the CXP using
the wrapper's definition path, its saved TaskID and an explicit record number.
Wrapper 0 (Humidity, number 1) matches 300 bytes at offset 1336 of the archived
UM. Wrapper 1 (Temperature, number 0) matches the next 340 bytes at offset 1636.
Using the uploaded declarations and ST bodies with the existing st-il project's
task, placement and settings produces the same compiler-side candidate as above.
Direct whole-project CXP linking remains open.

On 21 November the offline research builder reproduced the compiler image from
that CXP and the sample's settings.json. From the repository root run
`go run ./omron/internal/research/cpu21sample --readback omron/internal/research/cpu21sample/testdata/cpu21-upload.cxp NEW.n1`.
The six instance numbers and H allocations come from POUInstances. The extra
global declaration K (type K1) has no allocated instance or call. Both graphic
rungs supply their enable, instance name, CIO input, DM output and output coil.
The CXP supplies the task name, ID, startup flag, section order and textual
instructions. The noninitial END section accounts for the generated CMP marker.
The resulting 4672 bytes match the archived UM outside service words 0, 1 and 4.
The regression test predicts those fields from the build and checks the full
UM hash. This research command is scoped to the archived upload. General CXP
linking and inference of the external image settings remain open.

19 November: [native instance binding](omron-cj1-instance-binding.md) now
reconstructs4 complete3092-byte direct-reference images: used definition23words,
instance44words,taskFBICALL, current/check3.0/4.0. Four generated invocation
frames also match source compiler before full Compile, but full task checking
rejects the invocation recipe with9errors. This negative control stays closed:
parameter-stack/source-invocation and hardware qualification remain false.
No PLC traffic. Next resolve native generated-invocation lifecycle.


19 November: [program guard version](omron-cj1-program-guard-version.md) is now
observed separately from KBase current. Previous Compile(...,false) selected
constructor Program Check VersionFF/FF; prior byte equality does not itself
qualify whole-program version checks under3.0. New crossed matrix explicitly
sets program3/4:60 Compile traces and24 independently reconstructed N1 files.
Next referenced FB image must set both versions and retain this trace.
No PLC traffic; production UnitVersion/transfer qualification stays closed.


19 November: [explicit current3/current4 FB/settings comparison](omron-cj1-version3-fb.md)
completed: 1532 case records (100 native rejections) match prior qualified
baselines, and112 complete N1 files are independently reconstructed by Go,
including24 files with an unreferenced FB definition. Single-context FB
research probes now support explicit current3/4 in BCD. Next qualify the full
referenced definition/instance/task-call image; explicit production UnitVersion
and binary FB stay closed. No PLC traffic or writes.47 Python tests pass.


19 November: [explicit current3/current4 compilation](omron-cj1-version3-compile.md)
compares 5560 instruction results and 16 complete N1 files, byte-identical to
independent Go builds. Binary complete-image compilation now supports audited
textual resources without FB through API/CLI. Explicit UnitVersion3.0, binary
FB, full user projects and hardware write/restore remain unqualified. No PLC
traffic in this stage; portable corpus and raw capture replay retained.


19 November: [explicit local current3 profile experiment](omron-cj1-version3-library-profile.md)
shows default4.0 preserved, current3.0 set/read back, reset restored default,
FB support predicates1 and NOP unchanged on both models. Extended-protection
support predicate returns0 on tested3.0/4.0/4.1 ×arg0/1 for both descriptors.
This does not qualify full-image compilation or select0307/0317. Explicit codec
UnitVersion remains rejected; next compare instruction/full-image current3
oracles. No PLC traffic;43 Python tests passed.

19 November: [actual CPU manufacturing reads](omron-cj1-manufacturing-read-2026-11-19.md)
succeeded on both CPUs, twice through generic client and twice through new
ReadCPUManufacturingData API:26-byte copies agree, stable RUN/error0.
Observed native display3.0 on both, revision9 CJ1H /6 CJ1M. Full wire review
confirms only0501/0601/0226; Go tests/vet and41 Python tests pass.
PLC Information screenshots subsequently confirm actual Unit Ver.3.0 on both;
lot/PCB/revision/software details also match. Protected:No is a UI observation,
not a proven IsExtendedProtect mapping. No profile auto promotion, program
writes or transfer/restore qualification.

19 November: [CPU selector and Unit Version consumer](omron-cj1-manufacturing-consumer.md)
are now traced statically in extracted IOTableCS: CPU menu -> selector0/index0;
Ex475 callback DATA21..23 -> named Unit Version Number, formatted %X.%X/%X.
Offline inspector preserves native display and separate date guards;41 Python
tests pass. No0226 hardware capture yet; actual version and write/restore
qualification remain open.

19 November: [manufacturing response framing](omron-cj1-manufacturing-framing.md)
now binds standard Services6 and Ex Services9 SPMA vtables to normalization:
minimum DATA26, four command/end bytes removed, Ex callback bytes21..23 are
DATA offsets. Portable offline inspector/replay added;34 Python tests pass.
CPU selector and Unit Version semantics remain unknown; no0226 PLC traffic
or program writes. Full transfer/finalization/restore gates remain open.

19 November: [manufacturing-information route](omron-cj1-manufacturing-route.md)
now identifies services423/475/481 and a six-byte0226 command constructor.
Raw Unit/version overrides retained; CPU selector and response decoding still
need tracing. Deep mode explicitly enabled; static Craft analysis only.

19 November: [retained local profile state](omron-cj1-local-profile-state.md)
is now measured and reported separately in settings audit: both library profiles
have default/current bytes[4,0,0] and IsExtendedProtect0 after SetPLC.
Actual CPU/unit version, target protection/service and finalization remain open.

19 November: [typed0502 connection data](omron-cj1-connections-2026-11-19.md)
implemented and observed twice on both CPUs. CJ1H reports ETN21/DRM21/MAD42
number3; CJ1M reports ETN21/SCU41-V1/MAD42 number9. Raw wire verification
confirms read-only scope and stable RUN/error0. Basic I/O physical inventory,
protection/service/profile/finalization and write/restore gates remain open.

19 November: [actual parameter pairs](omron-cj1-parameter-pairs-2026-11-19.md)
succeeded on both CPUs: all eight area pairs match,16 files/59904 bytes. Raw TCP
verification confirms only0201/0501/0601, exact final flags and stable RUN/error0
status. These are supplementary parameter copies; full backup and write/restore
qualification remain open.

19 November: [parameter-area read API](omron-cj1-parameter-read.md) adds
bounded full-area0201 reads for PLC Setup, registered I/O, routing and CPU Bus
settings. Local protocol tests only; live double-copy capture and full backup
remain pending. No PLC commands were sent in this stage.

19 November, follow-up: [explicitly approved CJ1M mode-only trial](omron-cj1m-mode-only-2026-11-19.md)
succeeded. RUN->PROGRAM->RUN, release ACK, unchanged4672-byte UM verified by a
fresh connection. The4672-byte program was already present before control steps;
18 November's capture was4652 bytes. No program write. The earlier approval
block for this specific trial is resolved. Mode-only controls are now observed
on both real CPUs; full transfer/restore qualification remains false.

19 November: [controlled program session](omron-cj1-controlled-session.md) now
journals explicit acquire/STOP/status, optional execution, release and fresh
return access. All10 Omron packages, CLI and vet passed; local file-journal
cycles cover both models. Actual CJ1H/.37 mode-only trial succeeded:
RUN->PROGRAM->RUN, release ACK, fresh connection read exactly the same2716-byte
UM. No program-write attempt. General write/restore qualification stays false.
The first CJ1M execution was rejected by automatic approval review; subsequent
explicit approval and successful execution are recorded in the follow-up above.

[Mode/access wire API](omron-cj1-mode-access.md): explicit PROGRAM/MONITOR/RUN,
non-forced acquire and explicit global release implemented and tested locally;
correlated busy3001 retains the owner address. No implicit release/force/retry.
These helpers still need journal/session lifecycle integration before hardware use.
Read-only refresh of both real CPUs succeeded: RUN, two identical parseable N1
copies each (CJ1H2716 bytes unchanged; CJ1M4652 bytes), rawCPU/status stable,
fatal/nonfatal zero. Both Go backup reviews passed. All10 Omron packages,
Omron CLI and vet passed. No live control/access/program write commands sent.


Raw `research/hardware/20261118-cj1-mode-access-01`: 42 files,
SHA256SUMS `6cd59627a2fcce68086cab5d3e71f6a40823c7b33fc88519b53438ff3c711a05`. Contains read-only captures of both real CPUs; control APIs tested on loopback only.

Latest operator update (18 November): both CJ1H/.37 and CJ1M/.36 are back
on the network and disconnected from actuators, except LEDs and other
insignificant loads. This supersedes the earlier .36 offline/no-poll scope.
Isolation is operator-reported; availability was not probed for this update.
Both CPUs can enter read-only preparation; technical write/recovery gates and
all qualification flags remain unchanged. See [current scope](omron-hardware-verification-and-handoff.md).

[Program session coordinator](omron-cj1-program-session.md): exclusive Client
ownership, fresh PROGRAM-mode backup pair, synchronous write-ahead journal,
candidate/readback and separate one-shot original return on a fresh TCP client
are implemented and tested on loopback. No real PLC access this stage.
Mode/access lifecycle, target/protection/finalization, full backup and hardware
acceptance remain open; the API is experimental and no CLI download exists.
All10 Omron packages, Omron CLI and vet passed. Nine new top-level tests include
25 fault/configuration subcases. File Sync is not a universal power-loss
metadata-durability guarantee. A failed journal cannot resume or drive recovery.


Raw `research/hardware/20261118-cj1-program-session-02`: 12 files,
SHA256SUMS `40d38d7d9eb0343795d6c25def6db49a839fcff7ee06c3030cb733334a3ca6a5`. Localhost only; no PLC traffic.

[0307 block sender](omron-cj1-0307-block-sender.md): explicit mutating Go API now
correlates route/SID and exact full-block ACK; all post-exchange0307 errors close
the session without retry. Local TCP tested candidate/readback/original/readback;
Host Link528-byte framing tested over net.Pipe. No controller access this stage.
At this earlier block-sender stage session ownership and recovery coordination
were still absent; see the coordinator update above. No hardware qualification.

Block sender validation: all10 Omron packages, Omron CLI and vet passed;
additional fins tests passed after the bounded Host Link frame test was added.
Seven new top-level tests include10 malformed/error subcases, lost TCP ACK,
explicit mock byte return and528-byte Host Link. Raw
`research/hardware/20261118-cj1-0307-block-sender-01`: 17 files,
manifest SHA-256 `0530deccc3241032070230b90bfe0088e0973f24ebafe7f839e4b2c8234e4ef6`. Loopback only; no .37/.36 access.

[Fresh CJ1H backup review](omron-cj1-program-backup-review.md): read-only .37
capture produced two identical2716-byte UM files with stable CPU/status in RUN.
The new offline audit checks both snapshots and complete program copies; it does
not qualify restore/full backup. Minimal2632-byte candidate and original-program
return plans were prepared locally; no blocks sent. .36 was not contacted.
The cached-version writer studied here is a ProgChk dialog branch, not proved
physical unit-version acquisition. Sender/recovery remain the next implementation.

Backup/version-word validation: all10 Omron packages, Omron CLI and vet passed;
12 Python replays and9 backup-review CLI cases passed. Static replay checked3422
assertions over32 fully decompiled functions plus1 retained partial fragment.
Raw `research/hardware/20261118-cj1-current-version-fields-01`: 79 files,
manifest SHA-256 `ffc2e7330fe4afe28ab5c705222dbc83459f391c5507782acffd8927284a382e`. Controller access was read-only .37;
no PLC writes/mode changes, no .36 contact and no new vendor-code execution.

Operator update, 18 November: **CJ1H 10.0.174.37** has only LEDs connected;
all mechanisms that could be damaged or cause harm have been disconnected.
Stand actuator isolation is confirmed by the operator's report, not an independent
physical measurement. At that point CJ1M/.36 remained offline; the latest operator update above
now extends reported availability and isolation to both controllers.
The planned hardware stage remains conditional on the outstanding technical
checks. Write/restore implementation is incomplete; no write, mode change or
execution was performed. See [access status](omron-hardware-verification-and-handoff.md).

[Profile/version origins](omron-cj1-profile-version-origins.md): archived native
contexts bind selector8 to masks0x400 (CJ1M) and0x80000 (CJ1H-H), high0.
Default bytes initialize CurrentVersion but setters can change/reset it.
The source CS2SupportVersion stamp is retained separately; actual unit version
and active profile remain unqualified. Full source-reader decompilation timed
out; only a reviewed assembly fragment is included as partial evidence.

Profile/version validation: all10 Omron packages, Omron CLI and vet passed;
9 Python replay tests passed. Static verification checked3393 assertions,
with29 fully decompiled functions and1 reviewed partial fragment.
4 actual projects and5 synthetic audits matched expectations (NAUTILUS_CLI=1).
Raw `research/hardware/20261118-cj1-profile-versions-01`: 77 files,
manifest SHA-256 `ee6179b16a14f0e51a41aae40ff3ec42c0553b37f2c2224d51063f1d91a8c5bd`. No new vendor calls or PLC traffic.

[BR2/profile follow-up](omron-cj1-br2-and-fb-profile.md): the saved-memory reader,
writer and compiler bridge skip BR2. Its pinned default is DIFF8191,0, reported
as a conditional candidate. FB initialization tests support with DefaultVersion;
setting the saved-use flag tests CurrentVersion on the version-dependent branch.
Actual mask/version provenance and CPU profile remain unqualified.

BR2/profile validation: all10 Omron packages, Omron CLI and vet passed;
6 Python replay tests passed, with 3327 static checks against pinned binaries.
4 actual project audits and5 synthetic cases matched expectations (NAUTILUS_CLI=1).
Raw `research/hardware/20261118-cj1-br2-profile-01`: 57 files,
manifest SHA-256 `65c4b5d860f520641f0f5abc6b531cc172d573c0639a7ae38792b6db2876f086`. No PLC traffic or vendor execution.

[Stack source origin](omron-cj1-stack-source-origin.md): missing UseDefault means
true in the pinned reader. The bridge skips explicit stack values and retains
initialized defaults (table AC0/64/1536); false uses BitSize and WordSize+BitSize.
All four CXP omit UseDefault, so saved D0 is not an unconditional applied conflict.
Profile/feature activation, BR2 and initialization remain unqualified.

Stack-origin verification: 15 functions, 29 reviewed assembly checkpoints,
1496 static checks against pinned binaries; 3 portable Python tests passed.
All 10 Omron packages, Omron CLI and vet passed; 4 real and 5 synthetic audits
passed with NAUTILUS_CLI=1. Unforced standalone startup timed out, cause unresolved.
Raw `research/hardware/20261118-cj1-stack-origin-01`: 66 files,
manifest SHA-256 `6a1cada8c3c48b99ce13208b6abde750f2b7f8ac0729039ad95d7802c6e656bb`. No PLC traffic or vendor execution.

[Saved source-settings audit](omron-cj1-source-settings-audit.md) now inventories
FBMemory and declaration column5 on all four user CXP projects. Four saved banks
match the recipe text, BR2 is absent, and saved D0 differs from explicit AC0.
Initial data, conversions and full source-settings agreement remain unqualified.

Source-settings validation: all 10 Omron packages, Omron CLI and vet passed;
four real project audits and three synthetic controls matched expectations.
Raw snapshot `research/hardware/20261118-cj1-source-settings-01`: 39 files,
manifest SHA-256 `54efaf7cf8129ab7d5371a564e2c00b8af92439876b0b92d3488a8724e6e296c`. No network or vendor runtime.

Follow-up: [offline write-evidence audit](omron-cj1-offline-write-audit.md) now
checks request order, exact ACKs and readback, with terminal stop on failure.
It does not implement transport, live recovery or hardware acceptance.

Follow-up: [offline preflight](omron-cj1-offline-preflight.md) now decodes retained
CPU model/status and reported Kword capacity, and compares two N1 program copies.
Freshness, unit version, source-settings agreement and full backup remain unverified;
this is not online approval or hardware qualification.

[Local recipe reproduction](omron-cj1-build-reproduction.md) now verifies the
complete candidate by recompiling explicit source/settings/model/resource.
A successful in-process proof can pass candidate-build-provenance; saved reports
cannot authorize it. Historical origin and original settings remain unproven.

Reproduction verification: all 10 Omron packages, Omron CLI and vet passed;
17 standalone CLI invocations matched expectations (6 native-hash builds,
6 recipe assessments, 4 negative controls, 1 no-recipe assessment).
Raw snapshot `research/hardware/20261118-cj1-build-proof-01`: 66 files,
manifest SHA-256 `057462312f8080e7f25fd8ad9cfa1662de5b9ad2a788516ff49643ed7e13f7ef`. No PLC traffic or vendor runtime.

Preflight replay: all 10 Omron packages, Omron CLI tests and vet passed.
Eight standalone CLI cases passed their expected blocked/unverified outcomes.
Raw snapshot `research/hardware/20261118-cj1-preflight-01`: 49 files,
manifest SHA-256 `59d8325528276acb39942fd427a233724b4421cd98580d50aafea273b62cb25d`.
No PLC traffic or vendor runtime was used in this stage.

This report describes the current restored checkout and distinguishes portable
implementation, native-format evidence and hardware acceptance. CX-One, Frida
and Ghidra are research tools; production code and replay tests use Go only.

Follow-up: [static file-to-block byte path](omron-cj1-file-byte-path.md) now establishes
whole-file copying in the studied native branch. Target selection and CPU
finalization remain unqualified; hardware access is still read-only.

Finalization follow-up: [discriminating projects and candidate diagnostics](omron-um-finalization-boundaries.md).
The four historical readbacks cannot distinguish counting the24-byte prefix.
Two independently/native-matched projects now separate the studied rules;
actual CPU finalization and deployment remain unqualified.

## What is implemented

- CXP/CXT inspection/import, explicit CPU/timer/settings selection, qualified LD
  and scalar-FB encoding, task/FB linking and complete N1 generation through
  `naut omron compile-image`. Supported native recipes match byte for byte.
  This is a bounded compiler, not complete support for the Omron palette.
- N1 inspection, exact task-record extraction and qualified instruction decoding
  with preservation of unsupported tails. Readback is not restoration of original
  symbols, comments or graphical rungs.
- Read-only FINS UDP/TCP and Host Link operations, bounded UM reads and response
  validation. Prior CJ1H manual-download readbacks matched the independently
  compiled project except for CPU service words0/1/4. This is historical evidence,
  not a claim about the program currently on the PLC.
- A pure offline `plan-program-write` command: candidate0307 blocks, exact bytes,
  hashes and strict acknowledgement validation. It does not send or acquire access,
  change mode, finalize headers or qualify a deployable image.
- Plain CJ1 OBJ envelope APIs exist. OBJ-specific CLI wiring from older work is
  absent in this restored checkout; `compile-image` remains the supported CLI.

See [build scope](omron-n1-build.md), [hardware verification and handoff](omron-hardware-verification-and-handoff.md),
[service fields](omron-um-service-fields-and-write-plan.md) and
[CPU finalization](omron-um-native-sums-and-finalization.md).

## New native task-file evidence

The existing isolated instruction oracle now accepts an explicit file selector:
65535 for the whole image,256 for the owned task and257 for an absent-task control.
The launcher rejects other values, nonintegers and non-file/definition requests
before creating an output directory. The owned output path supports drive F.
No GUI automation or PLC API is involved. Each worker is disposable x86 PowerShell;
only the pinned local compiler DLL and injector are used.

Scope: CJ1M CPU21 and CJ1H-H CPU65, unknown unit version, explicit BCD, ordinary
unprotected task256, startup1, name `Probe`, CII=true, no FB. The two programs are
`END` and `LD W0.00 / OUT W0.01 / END`; all four task header options are false.

| Program | Full image bytes | Native task bytes | Result on both profiles |
| --- | ---: | ---: | --- |
| END | 2624 | 152 | Exact independent extraction match |
| LD/OUT/END | 2632 | 160 | Exact independent extraction match |

Eight separate files were emitted by the native writer. Each of the four
full-image/task-file pairs matches byte for byte. Both CPU profiles produce the
same bytes for these minimal recipes. These task goldens were not synthesized
by slicing the full images.

For selector257, the native writer returned1 with zero errors on both profiles,
but created an empty file. `ExtractN1TaskRecord` deliberately returns
`ErrN1TaskNotFound` instead. Empty files and these standalone records are rejected
as full N1 images. Neither successful native return nor matching task bytes proves
that a file is acceptable as a hardware task-download payload. FB dependencies,
auxiliary records, protected variants and task service selection remain outside
this evidence.

Portable corpus: `omron/um/testdata/native-task-file-cases.json.gz`, SHA-256
`7f2eae4675a5366295846f397aa5666bcd5b2274e6b301b36c623f01e6d13920`.
It preserves ten actual case events, requests, profiles and capture provenance.
Go tests compare extraction with separately emitted native bytes, check code,
profiles, lengths/hashes and missing-task behavior without loading any DLL.
Compiler SHA-256:
`a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141`.

Raw captures under `F:\cx-proger\research`, with manifest SHA-256:

| Directory | Files | Manifest SHA-256 |
| --- | ---: | --- |
| hardware/20261118-cj1-export-route-01 | 16 | e259aa4e0b07019e840ee887eba1205ab8a7309797a1bde81b58e3752d9dd01f |
| cj1-oracle/task-file-selector-20261118-01 | 19 | 585d9e543ad1a7295f9637f353fe7a184f82905663e0a576fea1620b34558fe2 |
| cj1-oracle/task-file-selector-20261118-absent-01 | 13 | 729d8b13e7ddc9184b6ea64baeb770961c970762c868f6393f81ede728e3f65a |

The first directory includes six rejected launcher controls:0,-1,65536,256.5,
string256 and a selector without file output. None created an output directory.
It also retains static evidence for the local exporter: CX-P command39253 maps
to124cbee0, which calls124c4310. That route reaches whole-image compilation and
OBJ wrapping; it is not evidence of selected-task GUI export. The isolated oracle
provided the required minimal task files without relying on that menu.

## Remaining work, in priority order

| Gate | Work and required evidence | Current blocker |
| --- | --- | --- |
| Full-image byte path | Static filename→whole-file bytes→transaction→0307 path is now verified for the studied branch. Qualify actual target/profile selection, supported input variants and capacity; keep OBJ handling explicit. | Offline research/implementation; can proceed now |
| CPU finalization | Resolve service words0/1/4 and the real64-byte versus simulator256-byte relation for explicit CPU/unit profiles. Preserve source image and report any target-dependent transformation separately. | Offline work plus later hardware confirmation |
| Transfer procedure | Offline request/ACK/readback audit and terminal failure handling are implemented. Retained CPU/capacity/program-copy assessment is implemented; 0307 block transport now validates route/SID and ACK; experimental direct-TCP coordinator now owns the session, saves a fresh program pair/journal and performs explicit original return; complete mode/access lifecycle, full backup and hardware qualification. | Offline implementation; actual writes require stand authorization |
| Project state | Audit initial values, retained data, settings, I/O tables and symbol provenance. State exactly which segments a program-only build changes and which require separate support. | Incomplete mapping; the earlier all-segments CX-One warning remains unresolved |
| First restricted stand test | Small supported program, exact CPU/version, backup and recovery prepared; explicit PROGRAM/RUN decisions, transfer and independent readback. Test both CPU families separately. | Both CPUs reported online and isolated from actuators except LEDs/insignificant loads; mode/access, target/protection and hardware recovery checks pending |
| Full product coverage | Broader instruction/addressing/modifier/FB/allocation matrix, diagnostics, target/project configuration, editor/LSP integration and Linux ARM/x86-64 regression. | Substantial implementation and qualification remain |

A selected-task download path is optional for the first complete-program test.
Its new minimal goldens do not replace the full-image byte-path/finalization gates.
The complete instruction palette is the final product objective; a restricted
stand test can use a clearly declared, already qualified subset.

## External access and next action

There is no external blocker to continued offline work. The isolated native
oracle works without GUI control; direct Ghidra analysis works without a Craft
Anything MCP connection. Missing GUI automation is not a compiler-research gate.

The controllers belong to a live installation: read only. Do not poll offline
10.0.174.36. No PLC was contacted for this stage. Hardware writes require the
user to confirm an isolated stand with actuators disabled and explicitly authorize
the intended write procedure. No such authorization has been given.

Next: qualify target/profile selection and CPU finalization using the now traced
complete-image service byte path, then
implement the offline transfer state machine and its failure/recovery checks.
No user action is needed for those steps. Retain CX-One as a stand recovery tool
until independent restoration has been qualified.
Raw `research/hardware/20261119-cj1-controlled-session-01`: 22 files,
SHA256SUMS `450a51e727d9fec81bcb666d9bb9c8a7959cbb7c36c3fa3ee74de5adef3eb8c4`. CJ1H mode-only controls executed; CJ1M execution rejected before process creation. No UM writes.
