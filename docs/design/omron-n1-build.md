# Independent CXP/CXT to N1 build

`compiler.CompileCXPImage` and `naut omron compile-image` generate a complete
offline N1 file using Go only. CX-One, Windows, Wine, vendor DLLs and an online
PLC are not used. This is the existing qualified FB/task subset, rather than
general support for every CJ1 project.

From the repository root:

```sh
naut omron compile-image -model "CJ1M CPU21" -resource 'Resource[0]' \
  -timer-mode bcd -settings omron/compiler/testdata/n1-example-settings.json \
  -o project.n1 omron/cxp/testdata/cj1m.cxp
naut omron inspect-image -function-blocks project.n1
```

PowerShell uses a backtick for line continuation, or the same command on one
line. Use `CJ1H-H CPU65` and the corresponding source for the other qualified
model. The destination must be new. Compilation and inspection finish before
the file is published; an existing destination is preserved. The JSON report
includes source/settings/image hashes, model, explicit settings and task offsets.

The example settings file is a reproducible research configuration, not an
automatic extraction of saved CXP settings. All fields are required: schema1,
four explicit Boolean options, five named banks with sizes, explicit stack,
definition name-to-number map and seven next-enable words. Unknown/duplicate
keys, case aliases, nulls, missing values and trailing JSON are rejected.

Current whole-image scope:

- BCD, unknown unit version, CJ1M CPU21 or CJ1H-H CPU65.
- Empty FB sections, scalar FBs, or the CPU65 In Out BOOL case below.
  Saved global placement, explicit definition IDs and
  zero next-enable state are required for the FB case. For no FB, supply
  `"definitionNumbers": {}` and the same seven zero next-enable words; all other
  settings remain explicit and validated. Unmatched FB symbols/records fail.
- Unprotected task IDs256..287, startup0/1, ASCII names, the qualified textual
  instructions and at most one supported graphical FB call per program.
- Zero header option flags and the explicitly supplied AC0/64/1536 stack.
  Lower-level header/memory APIs support additional independently tested states;
  their combinations with complete source projects still need qualification.

### CJ1H-H CPU65 and CJ1M CPU21 scalar BOOL controls

CX-Programmer F7 accepted this project at Unit Ver. 3.0 with 0 errors and
79 warnings. The five-cell call passes a BOOL value from W100.01 to instance
8 and copies its separate BOOL result to W100.02. The compiler uses the
ordinary BCD recipe and the saved instance placement.

```sh
naut omron compile-image -model "CJ1H-H CPU65" -resource 'Resource[0]' -timer-mode bcd -settings omron/compiler/testdata/n1-bool-control-settings.json -o control.n1 omron/compiler/testdata/cj1h-bool-control-unit30.cxp
```

The 4984-byte N1 has SHA-256 `1f29fa66b64fc843cbd27b89ad3e73de9bd9d44e65c851dbc94c225f691622d0`.
The independent CJ1M CPU21 F7 run also reported 0 errors and 79 warnings at
Unit Ver. 3.0. Its temporary file `BINC5CE.tmp` matches the CPU65 image byte
for byte. To reproduce the CPU21 result, select model `CJ1M CPU21` and source
`omron/compiler/testdata/cj1m-bool-control-unit30.cxp` in either command.
To reproduce the build through VS Code, create the workspace with the following command, open its folder, and run the extension's CJ1 build command.

```sh
naut omron new-project -model "CJ1H-H CPU65" -timer-mode bcd -check-unit-version 3.0 -source omron/compiler/testdata/cj1h-bool-control-unit30.cxp -settings omron/compiler/testdata/n1-bool-control-settings.json cj1h-control
```

The optional `programCheckUnitVersion` in `cj1.json` selects checks on known version-sensitive source constructs. The build report records that choice. The image settings still select the available encoders.
The regression test compares the complete image and independently varies
each pin address. This comparison covers offline compilation.

### CJ1H-H CPU65 Ladder In Out BOOL

The following build reproduces all 5004 bytes of the Unit Ver. 4.0 F7 capture.
Run it from the repository root with a new output path.

```sh
naut omron compile-image -model "CJ1H-H CPU65" -resource 'Resource[0]' -timer-mode bcd -settings omron/compiler/testdata/n1-inout-v4-settings.json -o probe.n1 omron/compiler/testdata/cj1h-inout-v4.cxp
```

Expected SHA-256 is `8a5cba1a0b147ec3dce7507311b75b9de4833857239d3596f5e15679156683d7`.
The recipe selects `inOutBOOLVersion` 4.0 for the single Ladder BOOL inversion.
The four-cell call in `HM` passes the address of `W100.01` to saved instance
`Probe1` (8), alongside the scalar `BLINK` definition. The regression test
changes the pin to `W100.02` and checks the resulting one-byte change in the caller.

Header word 6 gains bit `0x02000000` from a compiled In Out declaration.
The retained `ghidra-header-options-18.json.gz` shows
`SetUsingStringOrVarInOut` writing KBase offset `0x21c` at `120127ad`.
In `fb-writer-ghidra.json.gz` the writer reads the field at `121c5305`
and sets the bit at `121c5320`. Selecting the recipe version alone leaves
the bit clear for scalar-only definitions. Other In Out types, STRING code
generation and hardware execution remain open. CX-Programmer rejects this
declaration under Unit Ver. 3.0.

### Image layout

The compiler joins42 fixed header words,288 task-configuration words and the
FB/task resource body. Section offsets and total length are derived from the
generated components; the task directory retains directory-relative offsets.
Image task records have separately reported absolute offsets. Every occupied
saved instance region must fit its explicitly configured bank, in addition to
the existing pairwise instance-overlap and call/reference checks. A separate
N1 parser validates the resulting extents and FB wrappers before publication.

Tests reproduce both saved native files exactly:1148 words /4592 bytes using
their archived first-rung source recipes. The unchanged original CXP has a
different first rung and yields1151 words /4604 bytes. It is intentionally not
forced to match the shorter capture. Startup changes are confined to the task
configuration word; neither source documents nor settings are mutated.
The fixture recipes remain operator-reported native source provenance, not a
new full-CXP observation inside the vendor compiler.

The authored minimal `omron/compiler/testdata/n1-no-fb.cxt` also builds through
the same CLI, with an empty definition map. Its entire2624-byte output matches
both archived native END-only files: directory at word330, one task256 at618,
no FB section headers/offsets and zero emitted FB-memory fields. The example's
configured banks/stack are still validated but are not serialized without FB
definitions. See the adjacent `n1-no-fb-README.md` for original capsule and hashes.
This does not establish equivalence to the2880-byte program read from live CJ1H.

The standalone Windows CLI edit/build/inspect workflow was also exercised in
`research/n1-build-cli-01` outside Git. Its CJ1M file SHA-256 is
`52d783d09159eb2be3a76aa56dcf899bfd7d313da727107bcabe0a416b28eeb5`;
CJ1H is `8a88c6e726542537787cea1419ccc42a1d203a37efb2304218c2a4e1687ce4f3`.
Scoped Omron/research and CLI tests and vet passed. Linux ARM64 cross-build
passed; execution on Linux was not performed in this stage.

The output is linked into an N1 file (`linked=true`). It does not contain
generated initial variable data, provide a full global storage/operand audit,
establish exact CPU/unit capacity or execution compatibility, or qualify a
PLC transfer. The report keeps `storageInitialized`, `globalStorageAudited`,
`sourceSettingsVerified` and `hardwareQualified` false. No upload or connection
is attempted. Saved CXP StackSettings D0 is inactive under the studied missing-UseDefault
reader rule; the initialized table is AC0/64/1536. See
[stack selection](omron-cj1-stack-source-origin.md). Actual profile/feature
selection remains unqualified; AC0 in the recipe is still an explicit input.

Next work: qualify source settings and initialization, extend allocation and
instruction/graph coverage, integrate the build recipe with Nautilus/VSCode,
then verify controlled transfer, readback and execution on the actual CPUs.
