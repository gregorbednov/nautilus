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
- Nonempty qualified scalar FB definition and instance sections; saved global
  placement, definition IDs supplied explicitly, zero next-enable state.
- Unprotected task IDs256..287, startup0/1, ASCII names, the qualified textual
  instructions and at most one qualified scalar FB call per program.
- Zero header option flags and the explicitly supplied AC0/64/1536 stack.
  Lower-level header/memory APIs support additional independently tested states;
  their combinations with complete source projects still need qualification.

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
is attempted. Saved CXP StackSettings D0 versus observed compiler AC0 remains
unresolved; choosing AC0 in the settings file is an explicit compiler override.

Next work: qualify source settings and initialization, extend allocation and
instruction/graph coverage, integrate the build recipe with Nautilus/VSCode,
then verify controlled transfer, readback and execution on the actual CPUs.
