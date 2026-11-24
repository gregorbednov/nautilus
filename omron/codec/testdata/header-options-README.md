# N1 task header options: native matrix and independent encoding

`header-options-bcd-*` and `header-options-binary-*` each pin a five-asset
capsule using the existing `program-object-v1` probe. Each contains 16 input
combinations and 32 successful file observations (CJ1M CPU21 / CJ1H-H CPU65).
All cases construct task256, name Probe, startup1, code END, CII=true, through
normal object APIs. Unit version is unknown. No GUI, PLC or network operations
are involved; newly owned files are embedded verbatim in the event log.

Each case explicitly sets all four options and reads them back through normal
getters. No KBase/object/compiled-code fields are written by the observer.
The timer mode is set explicitly per run and its getters checked per profile.

| State | KBase field | Setter / getter | Header word6 bit |
| --- | --- | --- | --- |
| Independent IR/DR | 29c | 120128c0 / 120128e0 | 20000000 |
| Binary timer/counter mode | 2a0=0 (BCD=1) | 121cf6cb / 12012600 | 40000000 |
| Extended TIM Hold status area | 2a4 | 121cf715 / 121cf72e | 08000000 |
| Copy prohibited | 2ac | 12012620 / 12012660 | 00002000 |
| Overwrite prohibited | 2b0 | 12012640 / 12012680 | 00004000 |

These are separate settings: Extended TIM Hold is not extended password
protection (field2bc). In particular, the native default 08000000 is explained
by Extended TIM Hold=true; setting it false produces zero flags in BCD with
the other options false. Binary mode sets40000000, not00000200.
Ghidra18 corroborates setters/getters; the existing pinned writer export
121c28ef corroborates their header mapping. Raw displacement scan17 was only
used to locate candidates, not to infer receiver types or instruction boundaries.

`codec.EncodeN1TaskFlags` implements this bounded scope with a required explicit
timer profile. `TestN1TaskFlagsMatchNativeOptionMatrix` checks all64 complete
files word by word (656 words each). Every other header/configuration/directory/
task/code word stays unchanged. The eight earlier owned-program cases still
replay independently, including CII=false and the native default option state.

Reproduce by running `Invoke-CJ1InstructionOracle.ps1 -ProgramObjectProbe
-TimerCounterMode bcd|binary -Cases header-options-MODE-cases.json
-Output NEW_DIRECTORY`. Original raw runs: research/cj1-oracle/
header-options-bcd-01 and header-options-binary-01 outside Git. Replay needs
only repository fixtures and Go, without Windows or vendor libraries.

This does not qualify all header flags or generate a complete project image.
Task protection, flags derived from FB/code attributes, automatic allocation,
password buffers, FB/stack memory setup and initialization are outside this
API's scope. CXP provenance for these options is not yet established: the
saved CXT does not expose them as directly named scalar fields. Do not infer
them from a screenshot, the presence of FBs or the native default context.
The backend must receive explicit options until source mapping is qualified.
