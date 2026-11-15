# CJ1 function-block evidence and N1 prefix inspection

Research date: 2026-11-15. Observed binaries are pinned CX-Programmer 9.6.1.1
and CX-Pdata.dll SHA-256
`a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141`.
This describes saved BLINK captures, bounded inspection and standalone FB body
compilation, not a complete executable project or hardware qualification.
All offsets below are zero-based 32-bit word offsets
in the big-endian N1 file; event buffers instead contain x86 little-endian words.

## Prefix records

### Standalone task record generation

`compiler.CompileCXPTaskRecord` and CLI `compile-task-record -model MODEL
-program PATH -timer-mode bcd FILE` generate one zero-flag task record from CXP.
`cxp.SourceForTask` requires TaskID256..287, OpStart1, Protection.Enabled0 with no
additional protection fields, and an ASCII identifier name1..24 bytes. Body
compilation uses the qualified mixed path with section separators, at most one
scalar FB call and terminal END. Other classes/metadata fail atomically.

All188 words of task256 and176 words of task257 match both native fixtures,
using the first-rung source recipes described below. Unchanged original CXP
produces191 words for task256 because its body has154 rather than151 words.
The task header is32 words: number at0, name at1..8 packed three ASCII bytes per
word (high byte zero), total record length at26, header size32 at27, other words
zero in this scope. Entry is `[0, bbc00f43, 61440000+number, 65440000]`, followed
by body and final `bb800f48`. Length is body+37. Writer121c28ef corroborates all
these fields and the optional two-word IR/DR prologue only for IDs below256;
that task class/prologue is not enabled here.

Code keeps body-relative spans. TaskRecord.SourceSpans and Generated are
record-relative: header0..31, entry32..35, body starting36, return at the final
word. Every word is covered once. A source mutation to ID287/name24 bytes tests
number/name emission, including all eight packed name words.

New `task-properties-*` contains34 normal constructor/setter/getter observations
(17 per CPU), with ordinary NOP encoding separately retained. Program type is0,
default TaskID256 and descriptor task count288. SetTaskID12215f70 accepts -1
through289; rejects -2,290,65535, leaving256 unchanged. Thus setter acceptance
does not establish a valid directory index. All production IDs are below288;
GetTaskType121ca1ee reports type4 for the qualified256..287 range. Name getters
retain25 bytes but the writer truncates at24; Go rejects truncation. Protection
getter follows its setter, while record flags at program+200 remain0 even for
protection1: do not equate these fields. Protected tasks remain rejected.
Raw Ghidra `ghidra-task-properties-13.json.gz` is hash-pinned; setter bodies use
normal native objects, with no forged code or compiled flags.

This is a standalone observed zero-flag record, not full task configuration or
a fresh native writer capture. OpStart and protection/configuration tables are
not emitted. The report keeps taskConfigurationGenerated/storageInitialized/
linked/hardwareQualified=false. Task directory, global N1 header, project-wide
settings/resources, initial data and deployment remain pending.

### Complete mixed program bodies

`compiler.CompileCXPMixedProgramBody(profile, doc, program, sectionMarkers)`
and CLI `compile-program-body -model MODEL -program PATH -timer-mode bcd
-fb-calls -section-markers FILE` now compose textual rungs with the qualified
seven-cell scalar FB call. The original textual-only APIs still reject graphics.
Scope is at most one FB call per program, explicit BCD and interlocks closed at
each call and section boundary. Repeated FB calls, other graphs and cross-boundary
interlocks require separate native qualification. FB records remain separate.

All151 words of the first native task's code now match on both CPUs, including
the complete invocation and three section separators. The test reconstructs the
exact source recipe from the existing portable oracle manifest: first rung
`LD W0.15 / OUT W0.01` for CJ1M input-bit, `LD W0.00 / OUT W0.01` for CJ1H baseline.
Reconstructed CXT SHA-256 equals the manifest's c1881eb043d9acc409a635410e91db03cc3c109c447e3addd65d632bf1d4234e
and1df37113657da21d0a9b3f151c64cf9063b222f5b38f32b93b7ea860023b93b6 respectively.
This reuses the saved captures and their declared source identity; it is not
a new native full-project capture or stronger operator/file identity proof.

The original CXP first rung is LD L1_Key_start / UP / SET ON. It produces154
words; only its first five words differ from the two-word capture recipe.
The remaining149 words agree exactly. Do not label the unmodified original's
154 words as identical to the151-word native task. The second program remains
133 source words or139 with separators, unchanged through the mixed entry point.

Composition shifts both source and generated spans, retaining bindings and
maximum logic depth. Every word is covered exactly once. Calls/markers after
END, missing END, unsupported graphs/versions/profiles and active interlocks
fail atomically. This qualifies the observed invocation inside a whole task
body without an additional surrounding-code rewrite for these two captures;
other caller contexts still need controlled examples. The32-word task header,
four-word entry, one-word return and N1/task linking are not generated here.

### Definition section and combined FB fragment

`compiler.CompileCXPFBDefinitionSection(profile, doc, resource, numbers)`
generates all supported definitions in source order, with the observed count
and three literal zero words. All50 words match the native definition section
at330..379 on both CPUs. This header is corroborated by writer121c28ef: full
image mode writes iStack_a70, then three zero words, before definition records.
The existing hash-pinned writer export is the static evidence; this stage adds
no fresh native full-project capture.

`cxp.SourceForFBDefinitions` requires unique case-folded names and unique body
TaskIDs, and caps the set at1024 definitions. The caller must supply exactly one
record number per exact definition name, each unique and in0..1023. Record IDs
are not derived from TaskID or declaration order. Missing/extra bindings, duplicate
IDs, ambiguous names/body IDs, unsupported source and late errors fail atomically.
Empty sections must be omitted by a future image linker and are rejected here.

CLI: `compile-fb-definition-section -model MODEL -resource 'Resource[0]'
-timer-mode bcd -number BLINK=0 FILE`; repeat `-number NAME=N` for each definition.
`compile-fb-prefix` accepts the same controls and requires explicit
`-next-enable 0,0,0,0,0,0,0`. It composes definition and instance sections,
matching instance source references to generated definitions. Both sections must
be nonempty in this API. All166 words match the complete native FB fragment
at330..495 in both saved images. Fragment-relative section offsets are0/50;
record offsets remain relative to their respective sections.

Changing BLINK record number to7 changes only the record's number word, leaving
FBBN and FBBCALL body IDs unchanged. A two-definition source mutation, with
record IDs9/2 and body TaskIDs1024/1025, produces96 definition-section words and
212 fragment words; the second instance calls1025. These are generated-source
tests backed by existing primitive/writer evidence, not fresh native captures
of a multiple-definition project. Source order is the explicit deterministic
assembly policy; broader CX project ordering remains unqualified.

Reports keep linked/storageInitialized/hardwareQualified=false. The prefix
reports instanceDefinitionReferencesChecked=true for its internal name/path
bindings only; all callers, task IDs, project-wide resources and FB call versions
are not checked by this operation. Saved placement and explicit zero next-enable
state remain required. The fragment has no N1 file header, task directory or
task code and is not a deployable image or wire payload.

### Instance section generation with explicit next-enable state

`compiler.CompileCXPFBInstanceSection(profile, doc, resource, [7]uint32{})`
and CLI `compile-fb-instance-section -model MODEL -resource 'Resource[0]'
-timer-mode bcd -next-enable 0,0,0,0,0,0,0 FILE` generate the complete observed
instance section: eight header words plus audited records. All116 words match
offsets380..495 in both native fixtures. Record spans are section-relative:
offsets8/62, length54 each. Removing the second source instance/symbol produces
62 words and count1. Empty sections must be omitted by the future linker;
this API rejects them.

Ghidra identifies the seven fields as the array written by
`PLC_KBase::SetNextEnableAddress`121cf6e3 at KBase+2d7c+index*4. Constructor
121b96e5 explicitly zeros all seven elements. Archived writer121c28ef writes
instance count followed by these28 bytes, then individual records. Literal
displacement candidates are121b9ace (constructor),121c5b07 (writer),121cf706
(setter). This search is not proof against aliased accesses. Raw archives
`ghidra-section-fields-11.json.gz` and `ghidra-section-setter-12.json.gz` are
hash-pinned in codec tests.

`section-addresses-*` preserves74 observations (37 per CPU): normal setter calls,
before/after readbacks and clearing through the same setter; ordinary NOP
encoding is retained separately. There are70 successful setter calls for
indices0..6 and four rejections for -1/7. Values0,1,65535,-1,2147483647 are
preserved as32-bit words. Native acceptance does not establish address validity.
No object fields, code buffers or compiled flags are forged.

Production deliberately accepts only explicit all-zero next-enable state.
CLI omission, wrong arity, noncanonical integers and nonzero state fail without
partial output. State is not inferred from frame sizes or CXP. Mapping is
established; how project settings/allocation choose nonzero values remains
unknown. Evidence combines saved whole sections, normal setters and static
writer assembly, not a fresh successful full-project writer invocation.

JSON marks sectionGenerated/nextEnableExplicit and both instance audits true,
but globalPlacementAudited/storageInitialized/linked/hardwareQualified false.
Saved placement is still used. This is not a full N1 or a PLC wire payload.
Definition sections, task envelopes/linking, initial storage, nonzero-state
derivation and full-project/hardware qualification remain open.

### Resource-wide instance identity and placement audit

`compiler.CompileCXPFBInstances` / CLI `compile-fb-instances -model MODEL
-resource 'Resource[0]' -timer-mode bcd FILE` compile the complete saved global
instance set in source order, after an atomic audit. Every POUInstances row
must match exactly one global FUNCTION BLOCK symbol by name and definition,
with address `[Auto]` and flags `A`. Orphan FB symbols, case-ambiguous globals/definitions,
local FB symbols/records and local names shadowing these instances fail. Local
variable tables and local POUInstances lists must use the observed format.
The independent single-instance API intentionally retains its narrower scope.

After each frame's size/capacity validation, nonempty T/BR0 and H/BR3 intervals
are checked for pairwise overlap across instances. Bounds are half-open absolute
word ranges: the examples occupy T[3072,3074), T[3074,3076), H[512,515) and
H[515,518). Adjacent ranges pass; memory order may differ from source order.
Shared zero-sized C/BR1, BR2 and H/BR4 bindings occupy no interval. Errors return
no partial record set. Every generated record still matches the complete native
record on both CPU fixtures. Mutation tests cover overlap, late failure,
misbound symbols, orphan declarations, local instances and empty sets.

JSON distinguishes instanceIdentityAudited/instanceOverlapAudited=true from
globalPlacementAudited=false. This audit does not check scalar symbol/operand
collisions with private memory, all callers, task links, CPU-wide resources,
initial values or other resources. An empty instance set does not prove that
the resource contains no unresolved FB calls. It still requires a qualified CPU
and BCD profile. No new addresses or section header are generated.

The native writer121c28ef writes the instance section's count followed by
0x1c bytes from KBase+0x2d7c, independently of each instance's seven sizes at
program+0xf4. These are now identified as SetNextEnableAddress state, with the
explicit-zero section API described above. Do not replace them with summed
instance footprints or assume arbitrary projects always use zero.

### Independent scalar instance record generation

`compiler.CompileCXPFBInstance` and `naut omron compile-fb-instance -model MODEL
-instance 'Resource[0].GlobalVariables.POUInstances.Instance[0]' -timer-mode bcd
FILE` generate the observed runtime frame from CXP declarations and saved base
bindings. Both instances on both CPU fixtures match all54 record words: ten
header words and44 code containers, covered by17 generated spans. No executable
words are copied from a saved image. IDs, body TaskID and absolute bases come
from source; mutation tests change their corresponding operands.

The supported interface is two scalar word inputs followed by one BOOL input,
and one BOOL output, plus implicit EN/ENO. Allocation uses the existing offline
scalar rules. MemoryAllocated must be true, OnlineEditDiffExpand zero, sizes
must equal the logical frame and fit the private regions. The compiler uses
saved addresses; it does not choose placement, audit other instances for
overlap, validate all global symbol bindings, initialize stored variable values,
assemble sections/tasks or create a deployable N1. Initial values are not
emitted even when declarations contain them. Other interface shapes fail.

The frame is FBIN; five base assignments; LD FI/RSET FI/#SET ENO; explicit
inputs in reverse order; implicit EN input; FBBCALL; ENO and explicit output
transfers; FBIRET. Only this generated control sequence is qualified. Header
fields are instance ID, zero, total length and seven register sizes. The last
two are zero in this scope. Native writer121c28ef writes ID from program+e4,
zero, code length+10 and0x1c bytes from program+f4. SetBaseRegisterSize121d6276
accepts indices0..6. Constructor defaults and saved images corroborate zeros.

Evidence: `codec/testdata/instance-decode-*` contains four real PLC_Program
decodes in programType16 (two saved frames on two CPUs); `instance-compile-*`
contains110 normal instruction compiles,96 accepted and14 rejected. Raw Compile
results remain separate from measured SetCIIFlag(true) and SetCCFlags stages.
Generated moves/transfers receive CII; FBBCALL also receives CC flags. These
stages qualify70 component comparisons. The three FI/ENO bit controls use the
saved whole frames plus the programType16 final pass in hash-pinned
`ghidra-instruction-compile.json.gz`, not a successful fresh whole-program
compile. A normal program-object experiment timed out before cases and was
discarded; it supplies no qualification. `ghidra-instance-envelope-10.json.gz`
preserves InsertFBIN, SetBaseRegisterSize and SetFBBaseBodyNum evidence.

CLI reports savedPlacementUsed=true, storageInitialized=false,
globalPlacementAudited=false, linked=false and hardwareQualified=false.
The unchanged ordinary instruction probe, with explicit BCD mode, reproduced
all24 previous observations after adding the new research modes. Linux ARM64
cross-building is checked separately from execution; no hardware was contacted.

The two N1 fixtures have identical FB prefixes. Header word0x23 points
to definitions at330; word0x24 points to instances at380; the task directory
starts at496. The native writer in CompileMultipleProgramsAndProduceBinaryFile
121c28ef corroborates the layout: program flags0x20 select definitions and0x10
select instances. Definition headers are0x5c bytes; instance headers are ten
words. Preserved Ghidra output is `omron/um/testdata/fb-writer-ghidra.json.gz`;
provenance is checked by tests.

| Region | Word offset | Contents |
| --- | ---: | --- |
| Definition section header |330 | Count1 and three reserved zero words |
| BLINK definition record |334 |46 words:23 header and23 code containers |
| Instance section header |380 | Count2 and seven retained metadata words |
| First instance record |388 |54 words:10 header and44 code containers |
| Second instance record |442 |54 words:10 header and44 code containers |

Definition record fields:

| Relative word | Meaning established by writer/capture |
| --- | --- |
|0 | Definition number in low10 bits; remaining flags retained |
|1 | Reserved zero |
|2 | Complete record length, including23-word header |
|3..18 |64 name bytes, space padded; all bytes retained |
|19..21 | Reserved zero |
|22 | Packed port counts; BLINK has0x0402 |
|23 onward | Code including FBBN entry and FBBRET return |

The writer packs input count at program object+0xec into the high byte and
output count at+0xf0 into the low byte. SetNumberOfFBinputs1221b323 and
SetNumberOfFBoutputs1221b34b corroborate those object fields. BLINK's4/2 counts
are consistent with three explicit inputs plus EN and one explicit output plus
ENO. The inspector exposes raw `portCountsWord` without allocating ports.

Instance records contain instance number, a reserved zero, total record length,
seven retained count words and code. Both records have counts2,0,0,3,0,0,0.
Their numbers are0 and3, not consecutive ordinal indices. Uninterpreted header
and code fields are retained. The inspector requires observed entry/return
wrappers but does not treat internal relative operands as ordinary memory bits.

## Source allocations and relative body

Original CXP GlobalVariables.POUInstances explicitly retains:

| Instance | InstanceNum | BR0 | BR3 |
| --- | ---: | --- | --- |
| BLINK_L1 |0 | T3072,2 | H512,3 |
| BLINK_L2 |3 | T3074,2 | H515,3 |

BR1 is C3072,0; BR2 is #00009FFF,0; BR4 is H1408,0 in both instances.
Initialization code contains matching bases5000ec00/5000ec02 and
5000da00/5000da03. The two44-container buffers differ exactly at indices
1,3,13,18,19. Both match the old runtime GetProgramBinary observations in their
entirety, checked against hash-pinned CJ1H and CJ1M event streams. This qualifies
the observed correspondence, not a general allocation algorithm. H512/H1408
belong to this internal FB context; ordinary codec bounds have not been widened.

The23-container definition code consists of FBBN1024,19 body containers and
FBBRET. Representative relative sequences include:

* e23f3002,e27f3004,e25f3000 before the first timer.
* TIMH: bb4000c1,513c0000,653c3001 and
  bb4000c1,513c0001,653c3002.
* Timer-completion references e21f0000 and e21f0001.
* Relative outputs fa1f3003/fa1f3004 and SET/RSET f05f3003/f07f3003.

These agree with graphical source roles. A candidate layout places EN/ENO at
bits0/1, IN/OUT/ON at bits2/3/4, PV/PN at words1/2 and timers at relative slots0/1.
That layout is an inference from this single FB. General packing, alignment,
retained state and type rules are not implemented or qualified.

PouVarList codes0/1/13 must remain distinct from the separate nDATA_TYPE bitmask
accepted by InstructionInst::StringToDataType12175f7c. Definition cells are
comma-separated; program call cells use DC1. Neither is an ordinary VariableList.

## Internal primitives and post-encoding stage

Two new headless capsules preserve236 normal-context observations over both
CPU profiles:200 primitive observations (46 accepted) and36 identifier probes
(8 accepted). They are under `omron/codec/testdata/fb-*-manifest.json`, with exact
requests, transcripts, scripts and workers. Program type is0; no complete
project is compiled. They do not enable public codec support.

| Form | Observed normal-context containers |
| --- | --- |
| FBBN 1024 | b0400809,65440400 |
| FBBRET | bb800805,80000000 |
| FBIN 3 | b040080a,65440003 |
| FBIRET | bb800803,80000000 |
| FBICALL &0 &1024 | b2410806,61440000,65440400,80000000 |

Numeric syntax is context-dependent: FBIN #0400 emits65440190 (decimal400),
whereas ordinary #0400 means hexadecimal1024. FBIN &1024 is rejected in the
same probe; bare1024 is accepted. Generic literal parsing cannot be reused
blindly for internal identifiers.

The old CJ1M lifecycle log proves that per-instruction output is not final FB
code. The same FBICALL instance changes between EncodeFunction and Serialize:

| Stage | First word | Object flags | Remaining words |
| --- | --- | --- | --- |
| EncodeFunction | b2410806 |00080040 |61440000,65440400,80000000 |
| Serialize |927f0806 |000800c0 |61440000,65440400,80000000 |

Other observed changes include b2410140 to927f0140 and b000080e to9000080e.
The exact FBICALL transition is a regression control against the hash-pinned
old input-bit event stream. It establishes a later transformation, not a
universal rewrite formula or which routine performs it. GetBinaryCode1216f468
copies the existing buffer. Candidate research boundaries are
CreateTemporariesAndDoAsicFix121dea55 and DoAsicFixAndReturns121de87c; actual
Ghidra output is preserved in `fb-postencode-ghidra.json.gz`.

The next stage identified AddtoInvoke_Postamble1217170b as the invocation-specific
owner. It compiles the new instruction normally, then handles FBEND separately
as9000080e. Otherwise SetCIIFlag12175eb3 clears bit20000000 (only for a compiled
object); when the resulting word has class `(word & c0000000) == 80000000`,
SetCCFlags12175e61 applies023e0000. ResetCIIflag12175e8a is the unguarded clear
primitive. Other contexts, including DoPrePostAmble121dd444, use different
sequences; this is not a rewrite for every FB or ordinary instruction.

54 fresh `control-bits-v1` observations measure the two primitive operations
without altering baseline EncodeFunction words. All nine old trace changes now
match the independent Go postamble operation, including FBEND and contact forms.
Four `invocation-skeleton-v1` observations invoke normal GenerateInvocationCode
and GenerateEndOfInvocationCode1217166f on normally constructed objects, for
both CPUs and both boolean options. No compiled flag or code buffer is forged.
The native lists confirm OR CF115, optional BMOV PF0 ,IR62+, then the BMOV ,-IR62
PF0 / FBEND epilogue. The option's wider caller-state meaning remains unqualified.

GenerateEndOfInvocationCode emits `&%d` from the owned list count at object+48
plus one. The fresh unfinished skeletons produce headers3/2 for list counts2/1;
the captured complete call has nine instructions after its header and count10.
This counts logical instructions, not the26 code containers. The internal
`fbcode` assembler reproduces that26-container call using the five initial
encoded body controls. Private operand lowering is still supplied, not derived
from CXP: this qualifies the envelope, not complete source compilation.

The new raw capsules are under codec/testdata/control-* and invocation-*.
Their probe tags are checked against the manifest, literal injected request and
begin event; encode-only loaders reject a probe capsule. Ghidra caller/control
exports are hash-pinned under internal/fbcode/testdata. A cold encode-only smoke
run kept all54 baseline instruction results unchanged after the tool extension.

The call macro in task0 occupies code indices26..51 after LD ON at25. Its
FBCALL operand is10, while FBICALL carries instance0/body1024. Therefore the
FBCALL operand must not be assumed to be the saved InstanceNum.

## Inspector and next qualification

### Standalone definition record generation

`compiler.CompileCXPFBDefinition` and `naut omron compile-fb-definition` now
generate one definition record from source, including its entry/return code:

```sh
naut omron compile-fb-definition -model 'CJ1H-H CPU65' \
  -definition 'Resource[0].FunctionBlocks.FunctionBlockWrapper[0].FunctionBlock' \
  -number 0 -timer-mode bcd omron/cxp/testdata/cj1h.cxp
```

The result contains23 code containers and the complete46-word definition record.
Every word of that record matches both original N1 captures, including all23
header words. The generator derives name, relative frame, instructions, body
number and scalar port counts from CXP. The caller explicitly supplies the
definition number; the project-wide identifier allocator is still absent.
Number and source TaskID/body number remain distinct fields. Fifteen source
spans move past the two-word entry; two generated spans cover entry/return.
Body spans are relative to code, not the23-word record header.

Fresh `codec/testdata/definition-wrapper-*` evidence covers38 normal native
instruction compilations:14 successful FBBN/FBBRET cases and24 rejected controls.
FBBN uses a bare canonical decimal number0..2047; `&` syntax is rejected. The
entry is b0400809,65440000|number; return is bb800805,80000000. Public ordinary
instruction compilation still rejects internal FB primitives.

`definition-properties-*` contains16 further native controls using normally
constructed PLC_Program objects and real setters/getters. ProgramType32 yields
record flags0. SetFBNum writes+e4; the writer masks its low10 bits. Inputs and
outputs occupy+ec/+f0 and the setters reject66 but accept65. They also accept
negative counts; those malformed values are deliberately rejected by Go. The
writer packs counts as inputs<<8|outputs. Source counts include implicit EN/ENO.
The normal name setter retains65 bytes, while the writer copies at most64;
Go rejects long/non-ASCII names and numbers outside0..1023 to avoid silent
truncation/aliasing. Flags, alternate name encodings and other header variants
are not generated. Genuine property functions are hash-pinned as
ghidra-definition-properties-09.json.gz; the existing fb-writer Ghidra export
corroborates padding, byte order, reserved fields and total record length.

Changed-name/body-number/definition-number and port-boundary tests exercise
source-dependent output. These are component tests, not new full-project CX
captures or hardware runs. The output is an unlinked standalone record:
definition section headers, instance placement/initialization, task linking
and the full N1 image remain pending. The body-only command retains its19-word
output and does not acquire wrappers implicitly.

### Source-driven offline scalar definition body

`compiler.PlanOfflineFBAllocation` now assigns the bounded scalar relative frame.
`compiler.CompileCXPFBDefinitionBody` combines it with the strict graphical source
reader and relative codec. CLI:

```sh
naut omron compile-fb-definition-body -model 'CJ1H-H CPU65' \
  -definition 'Resource[0].FunctionBlocks.FunctionBlockWrapper[0].FunctionBlock' \
  -timer-mode bcd omron/cxp/testdata/cj1h.cxp
```

The command returns the frame table, memory footprint and19 machine containers
with15 source spans. Both original projects match the complete native body
between FBBN/FBBRET. The generator reads declarations and graphical cells, not
the saved N1. Swapping PV/PN declarations changes their timer operand words;
adding13 private BOOLs crosses a packing boundary and moves both word presets.
`relativeFrameAllocated` is true; instancesPlaced/linked/hardwareQualified stay
false. This body-only API omits FBBN/FBBRET; the separate record generator above
now supplies them. Instance initialization, absolute placement, task records
and the complete image are still absent.

Qualified rules: implicit EN/ENO occupy BR3 bits0/1. BOOLs are assigned by usage
input, output, internal, preserving declaration order within each usage. The
BOOL region rounds up to16-bit words; INT/UINT/WORD follow it in the same usage
order. Internal TIMERs use consecutive BR0 slots in declaration order. The raw
GetMemorySize result counts logical bits (BLINK: BR0=32, BR3=37); packed frame
sizes are two timer words and three data words. This does not establish CPU
instance capacity. Direct AT bindings, retained data, arrays, INOUT/external
variables, other types and online reserves are rejected. The CXP definition
reader now explicitly requires the observed five zero online reserve pairs.

The new native oracle uses a real `Omron.CXP.POUHeadDoc.Doc2` object with normal
AddVariable/GetAddressOffset/GetMemorySize/GetUsedBOOLAndWORDSize calls. PHeadDoc
requests online BOOL/OTHER reservation sizes from its owner through events10/11
on IIDd9e8bf87-d3ce-4964-aa4a-71aeb47a2d18. A document without an event sink can
return unreliable WORD offsets. The qualified offline owner supplies zero
reservations and explicitly disables online editing; it does not patch object
memory. Do not use the early unsinked smoke runs as allocation evidence.

`compiler/testdata/allocation-*` archives40 independent native documents: the
original declarations, their reversal/rename, mixed scalar usages/types and
BOOL counts around14/16/32-bit boundaries. Every binding, logical bit size and
packed BOOL/OTHER word size matches Go. This header allocation API is not CPU
specific; complete code comparison separately covers both qualified CPU models.
The capsule checks request hashes, actual loaded module hash, event settings,
method RVAs, lifecycle and input/result correlation. PHeadDoc SHA is
463eb4402bb6fa00453a5d7a745d77ced6be454d77aa2a683eb67de7e38ba71c.
Genuine Ghidra output is hash-pinned as ghidra-phead-allocation-08.json.gz:
COM RVAs6ac0/73e0/eb60 correspond to the three queried allocation methods.

### Independent short relative encoding

`codec.EncodeFBRelativeInstruction` and `codec.EncodeFBRelativeBody` now emit
the same short BR0/BR3 forms supported by the bounded decoder. Inputs are typed
relative instructions with explicit register/displacement/unit operands; source
span fields do not affect encoding. Offsets must fit -2048..2047, and TIMH
requires the BCD profile. Unknown forms, roles, banks, function qualifiers and
extended displacements fail atomically. No CXP variable allocation, wrappers,
instance records or task/N1 generation is performed.

The native context is now qualified for these forms. SetUseFunctionBlockFlag1
alone leaves all72 standalone OperandEncode results unchanged (`br-context-*`).
Calling the real InstructionInst::Compile1216b432, with a normally constructed
CStringArray and the exact explicit arguments used by PLC_Program::GenerateInst
121e3bde, accepts the rendered BR operands. The library performs its own operand
preparation and sets IsCompiled; no compilation flags or normalized records are
injected on this path. The research harness uses reviewed MFC42 ordinals541/801
(CStringArray constructor/destructor) and5861(SetAtGrow). It checks the actual
CX-Pdata constructor import against ordinal541 before invoking the array API.
Raw Ghidra and the bundled Ghidra ordinal map are archived and hash-pinned.

`codec/testdata/br-compile-*` contains292 native instruction compilations,146
per CPU, with FB mode and timer mode readbacks. 280 succeed and12 fail. The Go
encoder exactly matches all218 observations in its short-form scope. The other
74 cases include rejected syntax/banks and successful native forms deliberately
outside this scope. In particular, LD4095,BR3 emits a three-word macro, and
TIMH with extended displacements expands its operands. They must not be masked
into short offsets. Both saved BLINK bodies reconstruct all19 containers exactly
from their already allocated typed relative instructions.

These are successful internal instruction compilations, not full-program
compilations. The source-driven stage above supplies the qualified offline
scalar frame. Definition/instance records and task linking remain pending;
other types and placement policies are still unqualified. Ordinary
physical-address encoding remains separate.

### Independent relative body decoding

`codec.DecodeFBRelativeBody` now decodes the observed BR3 bit forms LD, LDNOT,
AND, ANDNOT, OUT, SET, RSET; LD of BR0 timer completion; and BCD TIMH(015)
with a BR0 timer and BR3 word preset. It consumes the body between FBBN/FBBRET,
returns typed register/displacement/unit references and word spans, and fails
atomically on unknown forms. It neither emits code nor allocates an instance.
Other banks, bit opcodes, timer modes and instruction forms remain unqualified.

The independent inspector exposes this with:

```sh
naut omron inspect-image -decode-fb-bodies -model 'CJ1H-H CPU65' \
  -timer-mode bcd omron/um/testdata/cj1h-baseline.n1
```

This includes the FB record inventory and `relativeBodies`. Word spans inside
each body are relative to its first instruction; `prefixOffsetWords` locates
that instruction in the N1 prefix (359 for BLINK). The original19 words decode
to the same15 instructions previously lowered from source. Connecting source
variables to these displacements is now covered by the bounded offline source
stage above; other allocation contexts remain unqualified.

The new native probe constructs PLC_Program normally, uses SetProgramType32,
calls Decompile(kbase,int-array,count), walks the normal AddToProgram list and
calls InstructionInst::GetText. GetBinaryText instead returns hex code and is
not a source decompiler. No compiled flags or internal decoded buffers are
injected. Runtime class size752 and list offset0x210 are corroborated by the
archived genuine Ghidra exports `codec/testdata/ghidra-*-0[1-4].json.gz`;
compressed/decoded hashes and function coverage are pinned in tests.

`codec/testdata/fb-decode-*` contains12 native observations: the actual23-word
BLINK definition and five explicitly declared low12-bit mutations, each decoded
on both CPUs. The original source hash/word offset and all changed indices are
verified against N1. Each decoded span must cover exactly the supplied words.
Native decoding confirms offsets15,16,2047,-2048,-1 and the original0..4.
Thus the observed displacement is a signed12-bit quantity. The mutated cases
are decoder evidence, not successful new compilations or executable projects.

The decoder renders IN as `2,BR3`, ON as `4,BR3`, EN as `,BR3` (zero), and the
first timer as `,BR0`. Separate native encoding experiments are archived as
`br-rendered-*` (72 observations) and `br-typed-*` (30 observations). Every case
is rejected by standalone OperandEncode, including address-prefixed variants.
Those earlier probes omit the preparation performed by the real Compile API,
which is covered by the short relative encoding stage above. No relative syntax
was enabled in the ordinary physical-address encoder.

### Symbolic definition body

`cxp.Document.SourceForFBDefinition` and `naut omron inspect-fb-definition
-definition PATH FILE` now read the observed CSV-cell definition format directly
from CXP/CXT. The result retains declarations, typed symbolic references,
implicit EN/ENO, comments and source locations. It contains no allocated addresses
or native code; CLI flags `compiled`, `addressesAllocated` and
`hardwareQualified` remain false.

The bounded layout is seven columns on one row: contiguous contacts, optional
trailing wires and one OUT, SET, RSET or TIMH(015) terminal. Contacts are ordered
by coordinates rather than serialized cell indices. Duplicate coordinates,
branches, gaps before later contacts, disabled networks, unknown fields/forms,
missing references and incompatible operand types are rejected atomically.
This does not qualify other graphical layouts or a general IEC type checker.

Both original BLINK definitions yield six networks and15 symbolic instructions:
LDNOT/ANDNOT/AND/TIMH; LD/TIMH; LD/OUT; LD/OUT; LD/AND/SET; LDNOT/RSET.
An independent test compares their opcode skeleton and instruction widths with
all19 body containers between the saved FBBN/FBBRET wrappers on both CPUs.
It explicitly masks operand fields; this proves order/negation, not allocation
or a byte-exact body compiler. Renamed declarations and shuffled cell storage,
malformed graphs and bounded cell fuzzing are covered separately.

A fresh24-observation native capsule, `codec/testdata/fb-body-*`, records a
failed physical-to-relative method. `FBBodyProbe` sets OperandEncode's program
type to32 and reads base registers through normal library APIs. Both profiles
report T3072/1024, C3072/1024, DIFF8191/0, H512/896 and H1408/128. H512..H515
bit operands and H513/H514 timer presets still fail with8955. T3072/T3073
contacts encode as ordinary absolute timer addresses, not the saved relative
forms. Eight cases succeed and16 fail. Merely selecting program type32 does
not reproduce native definition operands; ordinary H bounds remain unchanged.
The capsule verifier checks manifest/request/begin program type and profile
readbacks. A cold default-mode smoke retained the same24 physical results.

Next: inspect native FB operand translation/decompilation and qualify changed
definitions before implementing relative address allocation and body encoding.

### Standalone source call compilation

`compiler.CompileCXPFBCallBody` and `naut omron compile-fb-call-body` now generate
the observed seven-cell scalar call rung from CXP, including its LD enable.
The source plan supplies typed pins, bindings and identifiers; the generator
supplies native transfers, FBICALL and the qualified invocation envelope. Both
original projects produce all27 containers matching task0 indices25..51.
Every word has either a source span (six instructions/macros) or a generated
span (header/prologue/epilogue), and three symbol uses retain their provenance.

Fresh FBICALL evidence contains122 observations:98 canonical successful decimal
identifier combinations,16 canonical rejected bounds and eight noncanonical
syntax controls. Both identifier domains are0..2047;2048 is rejected on either
operand. This does not establish PLC resource capacity or prove the referenced
records were generated. Internal '#' syntax is distinct: #0400 encodes decimal400,
so `fbcode.InstanceCall` accepts integer IDs and emits only the '&' form.
The immutable capsule is codec/testdata/fbicall-*; exploratory runs with malformed
one-operand input were not used as qualification evidence.

Actual CX-P.exe code supplies the option without a heuristic: function12713de0
sets EBX to zero at12713e07, then pushes EBX at12713e40 as the last boolean
argument of GenerateInvocationCode. Call12713e4c reaches import thunk1285adf6,
which jumps through IAT12b227f4 to the named CX-Pdata export. The genuine Ghidra
caller export is hash-pinned as internal/fbcode/testdata/source-invocation-ghidra.json.gz.
Earlier native probes of both boolean values and the complete saved call
corroborate the false path. The broader meaning/preconditions of true in other
contexts remain unqualified.

The new command emits an UNLINKED body fragment. Definition/instance records,
allocation, timers inside BLINK, other graph layouts, whole-project type checks,
tasks and full N1 generation remain pending. Existing full-program and textual
APIs still reject graphics. This is source compilation of one call network,
not compilation of the complete FB or either complete supplied project.

### Confirmed declaration types

PHeadDoc.dll SHA256463eb4402bb6fa00453a5d7a745d77ced6be454d77aa2a683eb67de7e38ba71c
contains the source declaration enum PH_TYPE. An x86 LoadTypeLibEx(REGKIND_NONE)
read identifies BOOL0, INT1, UINT2, DINT3, UDINT4, LINT5, ULINT6, WORD7, DWORD8,
LWORD9, REAL10, LREAL11, COUNTER12, TIMER13, FUNCTION_BLOCK14, STRING15 and
STRUCTURE16;17 is the sentinel. No vendor COM object is created or registered.
The separate KBaseComp type library confirms native nDATA_TYPE bitmasks: BOOL1,
INT2, UINT4, WORD8, TIMER65536. Ordinal source codes cannot be passed directly
to native instruction methods. Naming all17 source types does not enable their
native lowering.

Actual Ghidra analysis of pinned CX-P.exe e91f15b8613e56c8f326bc83ce07f5a24b177f5f70a4a445e4eadfa34b0a9eb4
links the enum to file data:124895a0 invokes ICXP_POUHeadDoc2::GetItem3 through
vtable offset0x1b0 (432 in the x86 type library), carrying penumType into the row
constructor1248a3b0. The constructor stores it at record+0x10. Row writer1248a560
emits that field as the third column after usage and name; parser1248ab80 reads
the same slot. Thus original PV/PN code1 means INT; IN/OUT code0 means BOOL and
TIMER_1/2 code13 means TIMER. Usage3/4 is swapped by the CXP boundary; current
call planning rejects both, and this enum evidence does not enable INOUT.

`cxp.FBTypeName` identifies known codes while retaining the original numeric
value. `fbcode.ScalarTransferForPin` now derives supported scalar transfer types
from that identity, ignoring cached display names, and checks exact bindings.
All176 native scalar observations exercise the bridge. The full27-container
source-pin comparison now has no manual type map; FBICALL and invocation-option
selection are still explicit test controls. Complete graph compilation, project
type/range checks and allocation remain open.

Metadata/reader snapshots and three raw Ghidra exports are preserved under
cxp/testdata/phead-typelib.json, kbase-typelib.json, typelib-reader.ps1 and
pou-*-ghidra.json.gz. Tests pin hashes, engine/source provenance, enum names and
GetItem3's slot. Numeric type names appear in inspect-fb-call; the report's
declarationTypesIdentified flag describes identity only. compiled and full
typeCodesQualified remain false. GUI inspection was unavailable because the
node helper failed before initialization; none of this proof relies on GUI.

### Source call planning

`cxp.Document.PlanFBCall` and `naut omron inspect-fb-call -program PATH -rung
PATH FILE` now read the observed seven-cell, single-call scalar layout. This
separate source-plan API does not enable graphical compilation. It requires one
LD enable, one FBCALL, three inputs, one output and the measured connecting wire;
other geometry/operations fail atomically. Declaration counts, duplicate cells,
pin rows, instance type/number, definition identity and call-version agreement
are checked. Only saved global FB instance references are qualified for this
inventory; local-instance records and allocation are not implemented.

Serialized element order is not parameter order: the original IN cell precedes
the two numeric inputs in storage. Inputs at rows2/3/4 correspond to PV/PN/IN
in PouVarList input-declaration order; output row2 corresponds to OUT. The plan
retains all seven declarations, their raw numeric codes and source lines.
Local-over-global source bindings supply #100, #100,4.14 and W1.01; LD ON resolves
to W1.00. Saved source identifiers supply instance0 and definition TaskID1024.
No signedness or native nDATA_TYPE conversion is inferred from PouVarList.

A source-plan test feeds the plan into the previously qualified scalar/frame
primitives. Its initial explicit transport controls for type0/type1 have now
been replaced by the confirmed PH_TYPE bridge above. With an explicit FBICALL
recipe, all27 containers, including the LD before the call, match both saved N1
fixtures. This remains a controlled bridge test, not a complete source compiler.
The CLI reports compiled=false and typeCodesQualified=false. Corrupt geometry,
stale versions, count mismatches, aliases and unsupported metadata have negative
tests; cell ordering and source edits have separate regression tests.

### Scalar parameter generation

AddOperandPreamble121749ea is now independently measured through the research
`parameter-transfer-v1` probe:176 rows (88 per CPU) invoke the real method with
scalar count1, usage1/2, normally constructed parent/list objects and no forged
code buffers or compiled flags. The scope is BOOL plus WORD/UINT/INT; other
parameters use zero dimensions and empty formal metadata. No full project is
compiled. The probe retains the original FBCALL EncodeFunction result separately
from the generated child list, including type conversion and source read-back.

StringToDataType12175f7c returns BOOL1, INT2, UINT4, WORD8. This does not establish
mapping from CXP PouVarList numeric enums. Usage1 pushes the input; usage2 pops
the output. Word transfers use MOV and IR63, BOOL transfers BMOV and IR62:

| Typed transfer | Initial containers before invocation postamble |
| --- | --- |
| WORD input #0100 | b2410140,61400100,5517f009 |
| WORD output D101 | b2410140,6117f001,54010065 |
| BOOL input4.14 | b2410147,620c004e,5717e009 |
| BOOL output W1.01 | b2410147,6317e001,560de011 |

Actual generated children carry native flag40000000 and final opcode927f0140/47.
The typed flag explains why ordinary MOV's b240 is not the right initial
instruction here. `fbcode.ScalarTransfer` generates initial typed controls;
the existing invocation operation supplies the final transformation. Word
operands share public codec validation. Only CIO/W/H/D words and explicit
CIO/W/H BOOL bits are enabled; INOUT, arrays, BOOL constants, private addresses,
implicit ports and other types are rejected.

The26-container BLINK comparison now generates all four scalar transfers from
typed source operands. Selection/order and FBICALL instance0/body1024 are still
explicit test controls. CXP pin resolution, numeric declaration-type mapping,
allocation and wider caller-state qualification remain open. The source compiler
still rejects graphical FBs. Fresh raw artifacts are codec/testdata/parameter-*;
the Ghidra transfer/type exports are hash-pinned in internal/fbcode/testdata.

`N1Image.FunctionBlocks()` and
`naut omron inspect-image -function-blocks FILE.n1` expose the bounded layout.
Unknown prefix variants, counts, lengths, wrappers and ambiguous numbering fail
atomically; ordinary inspection still preserves opaque data through Original().
Returned slices are copies. Unknown internal instructions remain opaque.

Next: qualify type codes and graph pins, then reproduce relative allocation and
the body transfer instructions from source. Trace other post-encode contexts
as needed; the invocation-specific operation above is now reconstructed. The call
frame alone does not qualify its body operands or caller state.
Reproduce the complete call
from source. Independent FB generation must not copy executable sections from
the captured N1. New controlled full-project cases are still needed for changed
ports, multiple calls, empty/disabled calls, state and timers.

Validation: all scoped Omron/research and Omron CLI tests and vet pass. Bounded
N1 fuzzing and a dedicated FB-prefix fuzz target pass; the latter preserves the
outer image so malformed prefix records reach the new reader. The built CLI
reports BLINK,23 definition containers and two44-container instances. Linux
ARM64 ELF cross-build passes; Linux execution, new full-project CX captures and
hardware execution were not performed.
