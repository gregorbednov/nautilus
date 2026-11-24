# Selected native rung compiler

`CompileCXPImage(profile, document, resource, settings)` now links the qualified
FB prefix, task block, configuration table and fixed header into a complete N1.
`Bytes()` returns a copy of the immutable generated file; component diagnostics
retain local offsets and `Tasks` reports image-relative record offsets.
Explicit zero option flags, AC0/64/1536 stack, definition IDs and zero next-enable
state are required. Saved instance regions are checked against configured banks.
Both4592-byte native fixtures match exactly for their source recipes; unchanged
source produces4604 bytes. This does not initialize storage, infer opaque CXP
settings or qualify hardware. See [build usage](../../docs/design/omron-n1-build.md).

`CompileCXPTaskConfiguration` compiles only task metadata into the288-word
header table at word42. All cells match both native headers. Source OpStart0/1
controls bit4000 for present tasks; class/ID defaults remain in absent cells.
`CompileCXPResourceBody` includes this table as a separate result. Its Words
still exclude the global header; the full-image API composes it separately.

`CompileCXPTaskBlock` compiles every qualified task, rejects duplicate IDs/names
and emits a zero-initialized288-slot directory plus records in TaskID order.
Directory offsets and aggregate spans are relative to the block. The complete
652-word native block matches both captures. `CompileCXPResourceBody` adds the
FB prefix, checks call-to-generated-record links and reproduces all818 words
after the native global header. This fragment API returns configuration separately;
the full-image API above performs final linking for the qualified scope.

`CompileCXPTaskRecord` wraps the mixed sectioned program body in the observed
zero-flag task record. It reads qualified metadata through `cxp.SourceForTask`,
requires IDs256..287, startup0/1, no protection and ASCII identifier names1..24 bytes.
Record spans include header, entry, source/generated body code and return.
All188/176 words of the two tasks match both native fixtures using the corpus
source recipes; this is not full task configuration or complete image generation.

`CompileCXPMixedProgramBody(profile, document, programPath, sectionMarkers)`
composes text rungs with qualified seven-cell scalar FB calls. Two calls to one
definition may be adjacent, separated by one LD/OUT rung, or placed at the end
and start of consecutive sections. CLI: `compile-program-body -fb-calls -timer-mode bcd`
with optional `-section-markers`. Interlocks must close before calls and section
boundaries. It preserves generated/source spans and bindings, requires terminal
END and fails atomically for unsupported graphs and call layouts.
Full N1 builds include section markers by default. Set `"omitSectionMarkers": true`
in the image settings to omit them.

The reconstructed native source recipes reproduce all151 code words of the
first task on both CPUs. Original unchanged CXP emits154 words because its
first rung contains UP/SET instead of the capture's two-instruction replacement;
the remaining149 words agree exactly. The second task still gives133/139 words.
This generates no task header/wrapper, FB records, initial storage or full N1.
The existing textual-only APIs retain their graphic rejection behavior.

`CompileRungs(profile, []Rung)` validates explicitly bounded mnemonic networks
and emits the qualified codec containers in source order. `CompileCXPRungs`
adapts selected textual CXP/CXT rungs, preserving their native path/line numbers.
It reparses actual instruction text rather than trusting cached `Parsed` fields.
No Windows, vendor library, network or PLC is used by either API.

`CompileCXPProgramRungs(profile, document, programPath, rungs)` also resolves
exact operand names using that program's locals and its resource's globals.
Locals override globals, including when the local binding is unqualified: there
is no fallback to a differently typed/addressed global. The low-level APIs above
remain direct-operand-only. Unknown symbols and mismatched scopes fail atomically.

Supported bindings are explicit BOOL bit addresses and addressed16-bit
CHANNEL/WORD/INT/UINT/UINT_BCD operands. Native access bounds and CJ1 restrictions
still apply after substitution. Flags, auxiliary type metadata, automatic
addresses, alias chains, FB/array/structure bindings and NUMBER constants remain
unqualified. NUMBER conversion is context-dependent in CX-Programmer, especially
for BCD; it is not equivalent to copying an address or prefixing every value.
Lookup preserves exact name case. Case-variant collisions and names colliding
with native operand syntax are rejected pending qualification. No case folding,
substring replacement, member/index evaluation or implicit allocation is used.
Successful `Body.Bindings` audit each use's name, resolved address/type, lexical
scope, declaration line and source operand index (zero-based).

The compiler checks pending condition depth (maximum8), ANDLD/ORLD underflow,
unjoined blocks and outputs without their required conditions. Ordinary output
instructions need one condition; CNT/CNTX need count and reset conditions.
Counter continuation from either input is not qualified here: the counter ends
its block and a following operation must start its own condition.

TR must be defined before restore and may be assigned once per instruction
block. Normal LD after an output starts a fresh block; LD TR after an output
restores a branch point. LD TR while a condition is pending pushes another block
for ANDLD/ORLD instead of discarding the pending condition. TR saves retain the
current condition without consuming it. TR definitions and stacks cannot cross explicit rung
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

`CompileCXPProgramBody(profile, document, programPath)` compiles every declared
source rung in the selected program, including empty rung boundaries, and
requires an explicit terminal END. `cxp.SourceForProgram` checks the observed
FileType2, non-inline, IEC1131Type11/SecType0 layout, exact SC/RC counts and
indexed order. Unknown code fields, duplicate SL and unqualified flags fail;
graphics remain visible even alongside SL. Inspection Summary is not used as
an exhaustive program source. Scheduling metadata is not compiled by this API.

```sh
bin/naut omron compile-program-body -model 'CJ1M CPU21' -timer-mode bcd \
  -program 'Resource[0].Programs.Program[1]' omron/cxp/testdata/cj1m.cxp
```

The JSON format is `cj1-program-source-containers`, with `nativeTaskCompiled`,
`fullProjectCompiled` and `hardwareQualified` all false. Both original second
programs compile to 133 source containers. All 132 non-END containers match saved
task1 code indices 3..134, and END matches index 138. The two intervening triples
at indices 0..2 and 135..137 are b2400100,6140dead,6540dead. These have been
identified as the section separators below. The default source-only API excludes
them; the sectioned API generates them explicitly and never strips captured data.
The default remains a whole source body, not a native task or N1 image. The first programs
fail on the graphical BLINK call without returning a partial body.

Regression inventory covers all 41 nonempty textual rungs plus one graphical call
per original project; all 41 textual rungs compile individually in BCD mode.
Program0 has 21 declared rungs including four empty; program1 has 28 including
three empty. Neither API treats those source counts as native code sizes.

`CompileCXPSectionedProgramBody` and CLI `compile-program-body -section-markers`
add CMP(020) #DEAD #DEAD before each noninitial section. The pinned library's
GetSLForSectionSeperator factory explicitly returns CMP with those two operands
for the measured selector8 profile. This produces all 139 captured task1 code
containers exactly, for both original models. The report format becomes
`cj1-sectioned-program-containers`; task/project/hardware qualification remain
false because headers, entry/exit wrappers and N1 metadata are still absent.

`Body.Generated` attributes each separator to a section with its own container
offset/length; normal `Spans` retain only actual source instructions with adjusted
offsets. Every emitted word is covered once. User-written CMP still requires
an execution condition; a generated boundary is a separate lowering operation.
Empty sections after the first and interlocks crossing a section boundary fail
atomically pending more qualification. A single section needs no separator;
an initial empty section is covered by the original task1 captures.

The exact Ghidra factory/recognition exports under `testdata` are hash-pinned by
tests and require no vendor tools to inspect. They corroborate the signature;
the CMP library capsule and complete N1 task-code comparisons validate words.

Compile an unchanged supported symbolic rung from the supplied project with:

```sh
bin/naut omron compile-rungs -model 'CJ1M CPU21' -timer-mode bcd \
  -rung 'Resource[0].Programs.Program[0].Sections.Sec[0].ProgramData.R[3]' \
  omron/cxp/testdata/cj1m.cxp
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

The CLI automatically reads the selected program's symbol tables. It validates
declaration counts, uniqueness and list structure. Comments are opaque: their
quotes/commas and multiline raw blocks cannot become extra declarations.

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

Nine unchanged symbolic rungs per original project now match complete contiguous
sequences in the respective saved N1 task bodies: seven from program0, including
two complex TR networks, and two from program1. This exposed and corrected the
pending-condition LD TR rule above. The actual CLI compiled the original CJ1M
section0/rung3 to eight containers with four audited symbol uses; its original
project SHA-256 remains
`b819ef1bbc3e7179dccb021b382410b807e164d4fd5c1c07ac14e356ff4dcef0`.
UP/UP(521) and DOWN/DOWN(522) transform an existing condition without changing
stack depth and require a subsequent output. Their three/four-container macros
have independent library-oracle coverage. The original first rung now compiles
to five containers with two bindings; its UP occupies three of them. That first
rung was edited in the saved N1 and is not a complete original-source anchor.
Two other intact UP-bearing rungs now match the saved images. These are existing capture comparisons,
not new live CX ProgramCheck or hardware results.
The scoped Omron CLI tests pass. The broader Windows CLI suite still hits
previously recorded executable-name/path-separator failures and a stdout-capture
timeout outside the Omron tests; it is not a passing whole-application check.

`CompileCXPFBCallBody` / `naut omron compile-fb-call-body` now lower the observed
seven-cell single-call scalar rung separately from the textual APIs. The
definition/instance identifiers, PH_TYPE declarations, pin order and bindings
come from CXP; no native code is copied from an N1 template. The actual stock
CX-P caller selects invocation optionfalse, corroborated in Ghidra. FBICALL
uses decimal identifiers0..2047, verified against122 fresh observations.

```sh
naut omron compile-fb-call-body -model 'CJ1M CPU21' \
  -program 'Resource[0].Programs.Program[0]' \
  -rung 'Resource[0].Programs.Program[0].Sections.Sec[0].ProgramData.R[4]' \
  omron/cxp/testdata/cj1m.cxp
```

The27 emitted containers match the complete LD+call range in both saved N1s.
Six source spans and three generated spans cover every word exactly once;
three symbol uses retain declaration/use provenance. Failure emits no body.
This command produces an UNLINKED fragment: it does not generate the referenced
BLINK definition, initialized instances, native task or full N1. Only one LD
enable, three scalar inputs and one scalar output in the measured geometry are
supported. INOUT, arrays, other graphs and full assignment/range checking remain
unqualified. Textual APIs and whole-program compilation still reject graphics;
they do not silently concatenate this fragment into an incomplete executable.

This emits a selected instruction body only. Nautilus LD graph lowering,
general symbol/type allocation, full palette, graphical FB lowering, task wrappers,
native image generation and deployment remain unfinished. It cannot compile
either complete supplied project yet and does not silently omit their unknown
instructions or graphical FBs.
