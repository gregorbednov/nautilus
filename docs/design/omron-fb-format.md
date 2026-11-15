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

The call macro in task0 occupies code indices26..51 after LD ON at25. Its
FBCALL operand is10, while FBICALL carries instance0/body1024. Therefore the
FBCALL operand must not be assumed to be the saved InstanceNum.

## Inspector and next qualification

`N1Image.FunctionBlocks()` and
`naut omron inspect-image -function-blocks FILE.n1` expose the bounded layout.
Unknown prefix variants, counts, lengths, wrappers and ambiguous numbering fail
atomically; ordinary inspection still preserves opaque data through Original().
Returned slices are copies. Unknown internal instructions remain opaque.

Next: trace post-encode transformation with known object context, qualify type
codes and graph pins, then reproduce relative allocation and the call macro
from source. Independent FB generation must not copy executable sections from
the captured N1. New controlled full-project cases are still needed for changed
ports, multiple calls, empty/disabled calls, state and timers.

Validation: all scoped Omron/research and Omron CLI tests and vet pass. Bounded
N1 fuzzing and a dedicated FB-prefix fuzz target pass; the latter preserves the
outer image so malformed prefix records reach the new reader. The built CLI
reports BLINK,23 definition containers and two44-container instances. Linux
ARM64 ELF cross-build passes; Linux execution, new full-project CX captures and
hardware execution were not performed.
