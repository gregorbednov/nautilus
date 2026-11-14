# Selected native rung compiler

`CompileRungs(profile, []Rung)` validates explicitly bounded mnemonic networks
and emits the qualified codec containers in source order. `CompileCXPRungs`
adapts selected textual CXP/CXT rungs, preserving their native path/line numbers.
It reparses actual instruction text rather than trusting cached `Parsed` fields.
No Windows, vendor library, network or PLC is used by either API.

The compiler checks pending condition depth (maximum8), ANDLD/ORLD underflow,
unjoined blocks and outputs without their required conditions. Ordinary output
instructions need one condition; CNT/CNTX need count and reset conditions.
Counter continuation from either input is not qualified here: the counter ends
its block and a following operation must start its own condition.

TR must be defined before restore and may be assigned once per instruction
block. Normal LD after an output starts a fresh block; LD TR restores an existing
branch point. TR definitions and condition stacks cannot cross explicit rung
boundaries. AND/OR/NOT after an output refine the current condition. This covers
multiple outputs and branch restores without treating every LD as a new rung.

IL regions can span selected rungs but must close within the selection. Multiple
IL instructions may share one ILC; the result includes a `shared-ilc` diagnostic
because native program checking can report pair counts. A second ILC cannot
close an already cleared region. An unfinished condition before ILC is rejected.
These rules follow [Omron W474-E1-13](https://assets.omron.eu/downloads/latest/manual/en/w474_cs_cj_nsj-series_plc%2C_sysmac_one_nsj_series_reference_manual_en.pdf?v=9),
especially the logic-block, TR and interlock sections.

Any syntax, codec, structural or graphical-rung error returns a nil body with
a diagnostic path, line and code. Code is never partially returned, silently
skipped, reordered or substituted. Errors unwrap their codec/syntax/structure
cause. All existing model, unit-version, timer-mode and operand restrictions
remain in force. Limits are65536 source lines/rungs and8MiB aggregate text/path
bytes; these are implementation bounds, not qualified PLC capacities.

`Body` contains numeric `Words`, source `Spans`, `Warnings`, `MaxLogicDepth` and
`HasEnd`. `Bytes()` serializes the bare containers big-endian, matching observed
N1 code serialization. END is optional for a selected fragment; if supplied it
must be terminal. The compiler never adds END automatically.

## CLI

After rewriting a rung to supported direct-address mnemonics, compile it with:

```sh
bin/naut omron compile-rungs -model 'CJ1M CPU21' \
  -rung 'Resource[0].Programs.Program[0].Sections.Sec[0].ProgramData.R[0]' \
  /tmp/edited.cxp
```

Use `-timer-mode bcd` or `binary` when the selection contains timers/counters.
Repeat `-rung` for contiguous rungs in one native program. Source project order
is retained regardless of flag order. Unknown, duplicate, mixed-program,
noncontiguous and graphical selections fail before any JSON is emitted.
The explicit model selects the codec profile; this command does not establish
the source project's CPU/settings consistency or measured unit version.

The report format is `cj1-selected-rung-containers`, with source SHA-256, program
path, model/mode, body/source map and `codeHex`. `fullProjectCompiled` and
`hardwareQualified` are false. There is no download option or PLC connection.

## Verification and remaining work

Controlled first-rung replacements in both original CXP fixtures produce the
same two containers as the saved CJ1H baseline/CJ1M input-bit N1 captures.
The actual CLI also compiled the existing CJ1M input-bit experiment to
`e205e00ffa05e001`, retaining its source hash
`becb4b4c39cad9f8a336ecc61a91957923e69ee46c990effca9125b9eb49badf`.
Tests cover stack limits, source spans, TR lifetime, shared ILC, multiword
instructions, source/codec errors, refusal of graphical FBs and CLI selection.
Fuzzing checks atomic failure, bounded spans, exact source correspondence and
codec round trips. Complex flow checks have manual/test coverage, not a newly
captured full-program CX ProgramCheck result or hardware execution result.
The scoped Omron CLI tests pass. The broader Windows CLI suite still hits
previously recorded executable-name/path-separator failures and a stdout-capture
timeout outside the Omron tests; it is not a passing whole-application check.

This emits a selected instruction body only. Nautilus LD graph lowering,
symbol/type allocation, full palette, graphical FB lowering, task wrappers,
native image generation and deployment remain unfinished. It cannot compile
either complete supplied project yet and does not silently omit their unknown
instructions or graphical FBs.
