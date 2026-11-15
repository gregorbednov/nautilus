# CJ1 function-block evidence and N1 prefix inspection

Research date: 2026-11-15. Observed binaries are pinned CX-Programmer 9.6.1.1
and CX-Pdata.dll SHA-256
`a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141`.
This describes saved BLINK captures and a read-only inspector, not an FB compiler
or hardware qualification. All offsets below are zero-based 32-bit word offsets
in the big-endian N1 file; event buffers instead contain x86 little-endian words.

## Prefix records

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
