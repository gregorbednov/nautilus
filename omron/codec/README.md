# Native instruction codec — initial qualified forms

This package is independent Go code. It does not load vendor libraries or use
Windows, COM, Frida or a PLC. It emits numeric 32-bit containers for individual
instructions, not a complete program, N1 envelope or download payload.

Supported initial forms are LD/LDNOT, AND/ANDNOT, OR/ORNOT, OUT/OUTNOT, SET/RSET,
NOP, END and plain MOV/MOV(021)/MOV(21). Direct bit operands cover numeric CIO,
W, H, A, D and T/C completion flags. Address bounds and read/write restrictions are checked independently
of the network protocol. Symbols, indirect operands, modifiers, other families
and unqualified function-label spellings fail explicitly.

`Profile{Model: CJ1MCPU21}` and `Profile{Model: CJ1HCPU65}` currently describe
observed library-default configurations. Unit-version-specific profiles are
unqualified and rejected. These profiles do not authorize deployment or assert
hardware compatibility. Unit version and timer mode must be qualified before
the whole-project compiler is enabled.

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
from a machine operand alone. MOV consumes three containers; Boolean forms,
NOP and END consume one.

MOV supports direct word sources/destinations in numeric CIO, W, H, A, D,
T and C, and unsigned 16-bit `#` hexadecimal or `&` decimal sources. The radix
marker is retained in the encoding and decoded source. Signed literals,
indirect addressing, symbols and modifiers remain unqualified. A0..447 are
read-only destinations; T/C word operands are present values and may be written
by this measured MOV form. `WordAddress` and `Literal16` expose these distinctions.

## Evidence

The immutable fixtures in `testdata` contain 215 explicit cases for each CPU:
430 native library observations. Tests verify the hashes of inputs, exact
injected script/template/worker snapshots and compressed transcripts, then
validate lifecycle, profiles and case correlation before comparing Go results.
326 generated results match the codec, 82 negative cases are rejected and
22 cases remain outside its current instruction forms. Three common forms
are also anchored to the separately captured full-project N1 images.

A separate immutable MOV capsule adds 185 cases per CPU (370 observations):
288 encodings match, 72 invalid operands fail, and 10 indirect/modifier/label
cases remain explicitly outside the codec. It covers all seven direct word
areas, area-pair combinations, range boundaries and literal boundaries. All
174 common results from an earlier cold run reproduce in this expanded capture.

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
probes do not yet qualify a production timer codec or a unit-version profile.

Native IL spelling differs from monitoring addresses: numeric `0.00` is CIO;
the oracle rejects `CIO0.00`. Bare T/C in Boolean instructions mean completion flags;
bare T/C in `fins.Memory` mean present values. H512 is rejected in the measured
direct-bit context even though protocol memory addressing has a wider range.
These representations deliberately remain separate.

The library-default result is not hardware qualification. The complete target
still requires the full instruction/CPU/unit matrix, FB and task compilation,
image generation, download/read-back and execution tests.
