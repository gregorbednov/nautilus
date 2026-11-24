# Full FB files and fixed N1 header generation

The `fb-memory-file-*` capsule pins12 complete native N1 files: six memory
settings on CJ1M CPU21 and CJ1H-H CPU65, BCD, unknown unit version. The same
five valid settings and rejected81/80 stack split as `header-memory-*` are
used. Each file contains task256 Probe (startup1, END) and an unreferenced
definition0/body0 ProbeFB (one input/output including EN/ENO), with body
LD0,BR3 / OUT1,BR3. All instructions use normal GenerateInst/SetCIIFlag calls;
BR3 size1 is set through SetBaseRegisterSize. Owner binding and all other
properties use normal APIs; no object fields or compiled words are forged.

The key diagnostic is preserved verbatim: `0:Unreferenced Function Block Body -
ProbeFB`. Program::Compile returns0, with zero errors and one warning. The full
native writer returns1 with an empty error array and creates each file. A
scoped observer of Compile and the diagnostic-list append function records
the actual result and message. Neither hook changes arguments or returns.
The two lists are program+278/+294, counts284/2a0. Ghidra21/22/23 pin the
relevant checks and diagnostic path; 121f0487 emits message resource24c9 for
an unreferenced body. The older return0 was not proof of a failed full writer.

Definition objects are submitted directly to the full writer for their one
compilation. Compiling them first is not an idempotent preparatory operation:
the earlier raw diagnostic08 file has a duplicated FBBN prefix after a second
compilation. The controlled corpus checks that precisely one compile occurs,
with exactly six code words (one FBBN, two body words, one FBBRET envelope).
The caller supplies the source body, not explicit FBBN/FBBRET/END instructions.
PF0.x operands in the earlier experiment were invalid in this context; BR3
forms are independently qualified. This is a bounded pass-through definition
experiment, not a general POU or full-CXP frontend.

Every689-word file is reconstructed word by word using independent Go codecs:
42 fixed header words,288 task configuration words,33 definition-section words,
288 directory words and38 task-record words. Definition directory=330,
task directory=363, task offset=651; instance section is absent. The Go
relative encoder and definition record assembler produce the executable prefix
without copying it from native output. `EncodeN1FBMemory` supplies17 memory
words, including changed bank/stack values; the invalid split retains the
normal reset AC0/64/1536 state. This closes changed-memory emission checks
that were absent from the earlier no-definition corpus.

`codec.EncodeN1Header` now generates all42 fixed header words from explicit
profile/options, section lengths and optional configured memory. The existing
ordinary-file matrices still verify the no-definition memory omission. A
separate test verifies all42 words against both saved1148-word project captures
with explicit50/116/652 section lengths and AC0 memory settings. The public
API rejects invalid/overflowing layouts and unsupported profiles/settings.

Scope remains ordinary unprotected tasks and qualified simple scalar FB
records, with zero password buffers and no extra derived nested/string/
protection flags. This does not link a whole CXP project, initialize storage,
resolve CXP option/stack provenance or establish hardware compatibility. A
complete image builder must connect these independently generated components
and retain those outstanding acceptance gates.

Raw final run: research/cj1-oracle/fb-memory-file-02. Reproduce with
Invoke-CJ1InstructionOracle.ps1 -ProgramObjectProbe -TimerCounterMode bcd
-Cases fb-memory-file-cases.json -Output NEW_DIRECTORY. The flag
programDefinition=true selects this fixed two-instruction definition recipe
and requires explicit memory settings and file output. Replay needs Go only.
