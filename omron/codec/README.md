# Native instruction codec — initial qualified forms

This package is independent Go code. It does not load vendor libraries or use
Windows, COM, Frida or a PLC. It emits numeric 32-bit containers for individual
instructions, not a complete program, N1 envelope or download payload.

Supported initial forms are LD/LDNOT, AND/ANDNOT, OR/ORNOT, OUT/OUTNOT, SET/RSET,
NOP, END and plain MOV/MOV(021)/MOV(21). Direct bit operands cover numeric CIO,
W, H, A and T/C completion flags. Address bounds and read/write restrictions are checked independently
of the network protocol. Symbols, indirect operands, unqualified modifiers/families
and unqualified function-label spellings fail explicitly.

CMP/CMP(020)/CMP(20) compares two unsigned 16-bit values. Both operands are
read sources: direct CIO/W/H/A/D/T/C words or #hex/&decimal literals. Read-only
A0..447 and constants are valid in the second position. T/C here are present
values, not completion flags. Three containers preserve operand position and
literal radix. Indirection, @/% and immediate refresh remain unqualified.
The 212-observation capsule gives 96 exact encodings, 84 invalid-form refusals
and 32 observations outside qualified scope. The saved section separators decode
as ordinary CMP(020) #DEAD #DEAD. Signatures and variations follow
[Omron W474](https://files.omron.eu/downloads/latest/manual/en/w474_cs_cj_nsj-series_plc%2C_sysmac_one_nsj_series_reference_manual_en.pdf?v=1).

Rising (`@`) and falling (`%`) differentiation is supported for LD/LDNOT,
AND/ANDNOT, OR/ORNOT and SET/RSET with the same direct memory constraints as
their plain forms. Input variants use three containers; SET/RSET variants use
two. Modified TR operands, OUT/OUTNOT, branch forms, MOV and timers remain
unqualified. In particular, low-level `%OUT` output does not qualify that form:
W474 lists only immediate-refresh variations for OUT/OUTNOT.

Branch forms are ANDLD, ORLD, NOT/NOT(520), IL/IL(002) and ILC/ILC(003), each one
container with no operands. LD and OUT also accept typed `TemporaryRelay`
operands TR0..TR15. A TR operand is separate from a general memory bit or word;
other Boolean instructions reject it. This is instruction encoding only:
logic-block depth, TR lifetime and IL/ILC pairing require program validation.
The [selected-rung compiler](../compiler/README.md) now performs initial checks
for these rules and composes qualified containers; whole-project lowering and
image generation are still pending.

UP/UP(521) and DOWN/DOWN(522) are zero-operand condition transformers,
consuming three and four containers respectively. Their exact macros are
qualified; the internal field0x120000 is not a public bit address. Modified
forms and incorrect function labels are rejected even when the library emits
containers. The compiler requires both an incoming condition and a later output.

`Profile{Model: CJ1MCPU21}` and `Profile{Model: CJ1HCPU65}` currently describe
observed library-default configurations. Unit-version-specific profiles are
unqualified and rejected. These profiles do not authorize deployment or assert
hardware compatibility. Unit version and timer mode must be qualified before
the whole-project compiler is enabled. `TimerCounterMode: TimerModeBCD` or
`TimerModeBinary` selects the explicit measured mode for individual instructions.
Omitting it leaves timer/counter interpretation unknown and rejects these forms.

```go
instruction, err := il.Parse("LD W1.15")
// Handle err before encoding.
words, err := codec.EncodeInstruction(codec.Profile{Model: codec.CJ1MCPU21}, instruction)
// words == []uint32{0xe205e01f}; handle err before using the result.
```

The decoder handles one qualified instruction and returns its consumed word
count. Unknown instructions, reserved addresses and unqualified forms consume
zero words and return an error, allowing a caller to preserve opaque input.
Decoded source is canonical; original symbols and spelling cannot be recovered
from a machine operand alone. MOV consumes three containers; plain Boolean forms,
NOP and END consume one.

Timers/counters also consume three containers. BCD mode supports TIM, TIMH(015)
and CNT; binary mode supports TIMX(550), TIMHX(551) and CNTX(546). Bare names
are accepted for these six forms. Indices span0..4095; numeric indices and a
matching T/C prefix designate the same slot. Presets are direct word addresses
or literals. BCD requires #hex with every nibble0..9; binary accepts unsigned
#hex and &decimal. The decoder uses the explicit mode because the same numeric
containers can mean TIM or TIMX. Indirect and FB-relative operands remain opaque.

MOV supports direct word sources/destinations in numeric CIO, W, H, A, D,
T and C, and unsigned 16-bit `#` hexadecimal or `&` decimal sources. The radix
marker is retained in the encoding and decoded source. Signed literals,
indirect addressing, symbols and modifiers remain unqualified. A0..447 are
read-only destinations; T/C word operands are present values and may be written
by this measured MOV form. `WordAddress` and `Literal16` expose these distinctions.

## Evidence

The library emits direct DM bit operands even for the CJ1 profiles. W474-E1-13
sections3-3-1..3-3-6,3-4-1 and3-4-8 reserve these forms for CJ2, so the codec
rejects them in both directions. `ParseBitAddress` can still represent their
numeric address fields for research; that does not permit instruction encoding.
DM word transfers and timer presets remain supported. Source:
[Omron W474](https://assets.omron.eu/downloads/latest/manual/en/w474_cs_cj_nsj-series_plc%2C_sysmac_one_nsj_series_reference_manual_en.pdf?v=9).

The immutable fixtures in `testdata` contain 215 explicit cases for each CPU:
430 native library observations. Tests verify the hashes of inputs, exact
injected script/template/worker snapshots and compressed transcripts, then
validate lifecycle, profiles and case correlation before comparing Go results.
292 generated results match the codec, 80 negative cases are rejected and
58 cases remain outside its current instruction forms or documented CPU limits. Three common forms
are also anchored to the separately captured full-project N1 images.

A separate immutable MOV capsule adds 185 cases per CPU (370 observations):
288 encodings match, 72 invalid operands fail, and 10 indirect/modifier/label
cases remain explicitly outside the codec. It covers all seven direct word
areas, area-pair combinations, range boundaries and literal boundaries. All
174 common results from an earlier cold run reproduce in this expanded capture.

The branch capsule adds262 inputs per CPU (524 observations): 86 supported
encodings match, 386 invalid forms fail and52 modified/incorrect-label/foreign
mnemonic controls remain unsupported. All188 common cold-run results reproduce.
ANDLD, ORLD and LD/OUT TR0/TR1 also match containers in both existing full-project
N1 captures. NOT and IL/ILC currently have library-unit evidence only.

The modifier capsule adds454 inputs per CPU (908 observations): 496 generated
results match, 176 invalid cases fail and236 forms remain outside the documented
CPU or qualified operand/instruction scope. All492 common results match a prior
complete cold run. Actual Ghidra assembly/pseudocode for120662e3 and120c8ec7
corroborate the fixed helper literals. Existing N1 captures contain the input
helper/suffix with internal operand0x120000. The exact triple is now separately
qualified as UP(521), not as a differentiated direct address. Other uses of that
internal field remain opaque. It is not a modified-contact project anchor.

Three condition capsules (`condition`, `condition-bcd`, `condition-binary`)
preserve198 observations across both CPUs and all three mode configurations.
Each has10 qualified matches,20 invalid-arity refusals and36 observations outside
the supported spellings. DOWN includes the extra c3000000 container. Two intact
UP-bearing symbolic rungs per original project match complete saved N1 sequences;
the original first rung also compiles but was edited in those saved images.
Instruction signatures and lack of @/% variations follow
[Omron W474](https://www.omron-ap.com/data_pdf/mnu/w474-e1-13_cs_cj_nsj.pdf?id=2045).

The oracle calls the installed compiler's normal constructors, SetPLC,
Initialise, OperandEncode and EncodeFunction in a disposable x86 worker.
Object destruction is explicit. No user project or GUI is involved. This gives
known instruction inputs, but bypasses whole-program checks, allocation,
scheduling, FB lowering and relocation. `accepted` in the raw transcript means
only that this low-level sequence produced containers.

An important negative control: the low-level path emits code even for
`END(999)` and `LD(999)`. Consequently its success cannot qualify arbitrary
function labels. The Go codec accepts only the explicitly qualified spellings.
The previous TIM/TIMH initialization failures are now explained by the measured
binary timer default. Three new mode capsules (84 observations) verify normal
BCD/binary setters and getter readback: TIM/TIMH/CNT initialize in BCD mode;
TIMX/TIMHX/CNTX initialize in binary mode. MOV controls are unchanged. These
probes establish research configuration. A subsequent 840-observation operand
matrix now qualifies the six direct timer/counter forms: 252 encodings match,
540 invalid or wrong-mode forms fail and 48 indirect observations remain
unqualified. Seven numeric timer sequences also match each existing full-project
N1 capture; that corroborates containers, not the source's mode or hardware.
Unit-version profiles and full-program behavior remain unqualified.

Native IL spelling differs from monitoring addresses: numeric `0.00` is CIO;
the oracle rejects `CIO0.00`. Bare T/C in Boolean instructions mean completion flags;
bare T/C in `fins.Memory` mean present values. H512 is rejected in the measured
direct-bit context even though protocol memory addressing has a wider range.
These representations deliberately remain separate.

The library-default result is not hardware qualification. The complete target
still requires the full instruction/CPU/unit matrix, FB and task compilation,
image generation, download/read-back and execution tests.

Internal FB primitive research adds236 observations in separate `fb-*` capsules.
They remain outside this public codec: identifier syntax is context-dependent,
and captured FBICALL code changes after initial encoding. See the
[FB format investigation](../../docs/design/omron-fb-format.md). These observations
must not be treated as qualified ordinary instruction forms.

## N1 task header flags

`EncodeN1TaskFlags(profile, N1TaskOptions)` independently encodes the five
qualified header bits for ordinary unprotected task images: independent IR/DR,
binary timer mode, Extended TIM Hold, copy prohibition and overwrite prohibition.
An explicit BCD/binary mode and one of the two qualified CPU profiles are required.
All64 option/CPU/mode combinations match complete native minimal N1 files;
the portable replay checks every word. Options are explicit because their CXP
source provenance is still unresolved. This is not a complete global header:
FB-derived states, protection checks, allocation, passwords and memory setup
remain separate qualification work. See testdata/header-options-README.md.

## Configured FB memory descriptor

`EncodeN1FBMemory(profile, N1FBMemorySettings)` generates17 memory descriptor
words from five explicit native bank settings and a stack split in native
word-address units. Public getter results are verified for both CPUs; defaults
match the saved FB header. Native SetStackRegister rejects firstSize>totalSize;
Go also rejects unsupported banks, profiles, extents and noncanonical addresses.
This descriptor is not automatically written: without FB definitions the native
writer leaves header[15..31] zero. Controlled files prove that omission even
when FB mode and memory settings are enabled. Changed descriptors still need
full files with definitions, CXP provenance and storage/overlap qualification.
See testdata/header-memory-README.md for evidence and supported limits.
