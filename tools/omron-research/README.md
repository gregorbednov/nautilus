# CJ1 compiler research tools

These tools are research harnesses. Production Omron code does not depend on
Craft Anything, Ghidra, Java, Windows, COM or vendor DLLs. Nothing here contacts
a PLC. Keep original projects unchanged; use the generated copies for experiments.

## Portable evidence audit

`Read-CJ1TypeLibrary.ps1 -Binary PATH -ExpectedSHA256 HASH -Output NEW.json`
reads embedded type-library enums and function signatures with system
LoadTypeLibEx(REGKIND_NONE). It creates no vendor COM object and does not register
the library. Use x86 Windows PowerShell for native x86 vtable offsets. Saved
PHeadDoc/KBaseComp metadata and the reader snapshot under cxp/testdata corroborate
PH_TYPE and nDATA_TYPE as separate enum domains; actual CX-P Ghidra exports link
PH_TYPE to the source row. Existing output is never overwritten.

`oracle` reconstructs the saved source cases, validates artifact hashes and
compiler transcripts, and checks exact binary differences using repository
fixtures only. See [its usage and evidence limits](oracle/README.md).

```sh
go run ./tools/omron-research/oracle verify -root .
```

## Ghidra: selected functions and complete constructor catalog

`Invoke-CJ1Analysis.ps1` runs installed Ghidra headless with a process-local
Java environment, a 768 MB heap and one analysis CPU. It verifies the exact DLL
SHA-256, caches a local copy, rejects existing output, and validates output
provenance and completeness. It never loads the DLL as executable code.

Supply explicit Ghidra/Java installation paths and a NEW output JSON. The
workspace must not contain dot-prefixed path components (Ghidra rejects them).
Existing projects are opened read-only; selected-function disassembly and
annotations made by a later invocation are discarded. Whole-program analysis
is disabled; inferred types and calling conventions need assembly review.

```powershell
$research='E:\cx-proger\research'
$ghidra='C:\Users\Greg\AppData\Roaming\craft-anything\engines\ghidra\ghidra_12.1.4_PUBLIC'
$java='C:\Users\Greg\AppData\Roaming\craft-anything\engines\jdk\jdk-21.0.12.1+1'
.\tools\omron-research\Invoke-CJ1Analysis.ps1 -GhidraHome $ghidra -JavaHome $java -Workspace "$research\ghidra-tool" -Output "$research\functions-new.json" -Addresses '1216d735','1216f468','121d68cf'
.\tools\omron-research\Invoke-CJ1Analysis.ps1 -GhidraHome $ghidra -JavaHome $java -Workspace "$research\ghidra-tool" -Output "$research\catalog-new.json" -CatalogFacts "$research\compiler-facts.json"
```

Function addresses MUST be quoted strings: PowerShell otherwise interprets
addresses such as `120662e3` as decimal scientific notation.

`ExportCJ1Functions.java` emits assembly, resolved calls and decompilation for
each requested function with the actual Ghidra version and input hash.

`ExportCJ1Catalog.java` statically recovers a constructor argument prefix from
each previously identified call site. It handles stack cleanup and the two
array builders without executing them. Every record must agree with prior
operand count, both family masks and all six handler addresses. Builder calls
recover literal template arrays at CInstruction +10C/+110/+114. Their element
widths follow the inspected builders, not a mnemonic's function number.

A length mismatch is recorded as `matchesDeclaredBaseLength: false`; it is not
silently repaired. All families in the DLL remain present. A recovered template
is not evidence that an instruction is supported by a selected CJ1 CPU, nor
that the template by itself is a complete machine instruction or UM image.

The earlier `inspect_cxone.py` extractor remains usable on Linux with objdump.
Its original inventory provides the independent call-site and metadata checks.

## Instruction-unit oracle without GUI

`Invoke-CJ1InstructionOracle.ps1` launches an owned disposable x86 worker and
calls the pinned compiler library's constructors, SetPLC, Initialise,
OperandEncode and EncodeFunction. It never attaches to a user CX-Programmer
instance or calls PLC APIs. Frida and the DLL are checked against the reviewed
hashes; the worker and injector are bounded and cleaned up. Native object
destructors run before completion is recorded. This research harness executes
vendor code and belongs only on the Windows oracle host.

```powershell
.\tools\omron-research\Invoke-CJ1InstructionOracle.ps1 -Cases .\omron\codec\testdata\library-cases.json -Output E:\cx-proger\research\cj1-oracle\instruction-new
```

The output preserves input cases, exact injected script/template/worker,
event stream, stderr and provenance hashes. The portable capsule used by Go
tests is under `omron/codec/testdata` and needs no vendor software to verify.

The `fbicall-*` capsule has122 rows over both models: a decimal identifier
matrix, negative/overflow cases and syntax controls. Both operands reject2048;
the internal Go encoder qualifies0..2047 only, without asserting PLC capacity.
`#0400` is decimal400 in this internal instruction, so it must not be passed
through the public hexadecimal literal grammar. The source caller's false
GenerateInvocationCode argument is separately corroborated in actual CX-P
Ghidra output, not inferred from EncodeFunction success.

Each case's `operands` must be a JSON array, including `[]` for zero operands.
Null, missing and scalar operand collections are rejected before output creation
or worker launch. A PowerShell conditional producing no pipeline values can
serialize as null; explicitly assign `@()` when generating zero-operand cases.

`-TimerCounterMode library-default|bcd|binary` optionally invokes the reviewed
normal timer-mode setter after each SetPLC and records both getter results.
The default leaves the library setting unchanged (measured binary in the saved
probe). For a BCD probe, use `-Cases .\omron\codec\testdata\timer-bcd-cases.json`
with `-TimerCounterMode bcd` and a new output directory. All three mode capsules
are portable; none changes the Windows installation or a user project.

Three explicit research switches extend the normal path:

* `-ControlProbe` tags the capsule `control-bits-v1`, preserving baseline words
  and then calling normal ResetCIIflag / SetCCFlags methods. It records both
  snapshots separately. These operations do not imply public instruction support.
* `-ParameterProbe` tags it `parameter-transfer-v1`. Each case is FBCALL &0
  plus `parameter: {type: "BOOL"|"WORD"|"UINT"|"INT", usage: 1|2, source: "..."}`.
  It calls actual StringToDataType and AddOperandPreamble on a normal object,
  with scalar count1, zero dimensions and empty formal metadata. It records the
  native type, source read-back and normally compiled child list separately from
  baseline encoding. Inputs are deliberately bounded; this does not invoke
  full-project checks or establish CXP declaration-type/pin mappings.
* `-InvocationProbe` tags it `invocation-skeleton-v1` and requires FBCALL &0
  cases with an explicit JSON `invocationOption` boolean. It invokes normal
  GenerateInvocationCode / GenerateEndOfInvocationCode, recording the two owned
  CObList contents and deferred-header snapshots. Constructors/destructors and
  native compilation are used; no compiled flags or code words are injected.

The switches are mutually exclusive. Fixture verification checks probe tags in
the manifest, literal request and begin event, and correlates probe inputs.
Encode-only loading refuses tagged probes. Existing encode-only behavior is
unchanged. See [the FB investigation](../../docs/design/omron-fb-format.md).

This is a low-level instruction oracle, not whole-program compilation:
programType=0 and unit version remains unknown. Timer mode is explicitly
recorded; the default run leaves it at the library default. It skips
program checks, overload fallback performed by higher-level compilation,
symbols, allocation and relocation. Both positive and negative outcomes are
limited to that call sequence. It even emits containers for misleading labels
such as END(999), so success must not be promoted to source-program acceptance.
Keep separate full-project binary comparisons and later hardware validation.

## Differential source corpus

From the repository root with portable Go on PATH:

```powershell
go run ./tools/omron-research/corpus -source omron/cxp/testdata/cj1m.cxp -o E:\cx-proger\research\cj1m-cases-new -rung 'Resource[0].Programs.Program[0].Sections.Sec[0].ProgramData.R[0]'
go test ./tools/omron-research/corpus ./omron/...
```

Each case is a copy of the WHOLE project with one selected textual rung
replaced. These are not standalone minimal programs. Graphical rungs, FBs,
other tasks, unknown fields and original CPU settings remain present. The
CJ1M settings disagreement remains a warning. The tool refuses graphical rungs
and existing destinations and publishes the corpus only after completion.

The manifest records CPU declarations, warnings, CXP/CXT hashes, source and the
changed feature. Cases cover direct addresses, area changes, modifiers, MOV
literals, timers and a branch. Modifier acceptance is deliberately unverified.
Compare move-decimal with move-hex and timer-fast with timer; other variants
compare with baseline. The untouched original is included as an import bundle.

Next, open copies in CX-Programmer, record resolved CPU/unit version, capture
compile acceptance/diagnostics and the ACTUAL executable output, and retain
hashes and the exact extraction method. Generated CXP/CXT is source only.

## Bounded runtime observation

`Capture-CJ1Oracle.ps1` attaches the portable, hash-pinned Frida 17.26.3 x86
injector to an already running CX-Programmer. It also verifies the compiler
DLL hash. The injector lives outside this repository under
`E:\cx-proger\tools\frida-17.26.3`; its official release archive SHA-256 is
`2f8cf99b99df64a075ab654514550e86c9e91a115db9696fe50806fa429240c5`.
No machine installation or settings change is needed. Attachment may require
a terminal running as the same Windows account/integrity level as CX-P.
Build its auditor first with
`go build -o bin/cj1-oracle.exe ./tools/omron-research/oracle`.
The launcher rejects an incomplete audit before extracting accepted binaries.

```powershell
.\tools\omron-research\Capture-CJ1Oracle.ps1 -ProcessId 4288 -Project E:\cx-proger\research\cj1-oracle\cj1h-experiments\baseline\project.cxp -CPU 'CJ1H-H CPU65' -Output E:\cx-proger\research\cj1-oracle\capture-new
```

Use the actual CX-P PID; omit it only when exactly one instance runs. The
output directory must be new. During the following 180 seconds select the
intended PLC in that instance and press F7. Retain the resulting compile log.
The observer logs CPU resolution, per-instruction buffers, assembled task
buffers and binary-file compilation entry/return, if these boundaries run.
It handles a DLL loaded after attachment, uses module-relative addresses,
and never invokes native compiler functions or replaces arguments/results.
Hooks detach after 180 seconds; the launcher then stops only its own injector.

For testing the complete postprocessing path without running CX-One or Frida,
pass `-ReplayCapture OLD-DIRECTORY` together with the original `-Project`,
`-CPU` and a new `-Output`. Replay verifies the recorded source/observer/event
hashes and retains its origin in metadata. It does not create a new live oracle.

The observer also reads the output file after a successful compiler return.
Each file is recorded as bounded hex in the event stream and saved as a
separate `compiler-output-SEQUENCE.bin` with its SHA-256. Serialization events
observe cached instructions too. A copy of the exact observer is retained.

`events.log`, `stderr.log` and `result.json` retain raw observations, errors,
hashes and capture status. Declaring a CPU/project to the launcher does not
prove which project the process actually compiled. The manifest deliberately
retains that distinction. A ready event alone is not compiler acceptance.
Instruction/program buffer events contain x86 little-endian memory bytes.
Compiler-file events preserve the serialized file bytes, observed big-endian
in the captured N1 images. PLC transfer framing and hardware execution remain
unverified. No PLC connection or transfer is performed.
