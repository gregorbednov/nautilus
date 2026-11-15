# Owned program object and native file writer

The `program-object-*` capsule records eight observations from a disposable
32-bit worker: CJ1M CPU21 / CJ1H-H CPU65, END / LD W0.00 + OUT W0.01 + END,
and CII false / true. Timer mode is explicitly BCD; unit version is unknown.
The manifest pins five assets and the injected request/template relationship.
Each file is embedded as exact hex in its case event, so replay needs no vendor
DLL, Windows, original output directory or external N1 file.

Reproduce with `Invoke-CJ1InstructionOracle.ps1 -ProgramObjectProbe
-TimerCounterMode bcd -Cases program-object-cases.json -Output NEW_DIRECTORY`.
This is a research tool requiring the pinned local vendor DLL and Frida.
It performs no GUI, network, PLC or license operations. Outputs are newly owned
local files. Raw run: `research/cj1-oracle/owned-program-04` outside the repo.

Construction uses normal PLC_Program ctor/SetType(0)/SetTaskID(256)/
SetStartUpFlag(1)/SetName("Probe") calls. MakeTaskConfigurationTable binds
the KBase owner at program+0x248 as a side effect. GenerateInst creates real
instructions; optional SetCIIFlag(true), AddToProgram, Compile and
GetProgramBinary produce the code. Object fields are read, never forged.
GetProgramBinary returns an end cursor, not an output length argument.
CompileMultipleProgramsAndProduceBinaryFile receives a real CPtrArray with
one program, an error CStringArray, mode=1, option=0, task=65535, option=0,
and an explicitly owned output filename. Every writer return is 1, with no errors.

`TestOwnedProgramNativeCompileAndFile` independently checks every N1 word:
330-word header, 288-word task directory, 38/40-word task record, 656/658-word
file. Source code matches the existing Go codec when CII=true. GenerateInst
clears CII, yielding 9b/c2/da instead of bb/e2/fa in these forms when false;
the probe changes CII only through the normal setter.

This is not a new full-CXP compilation capture. `fullProgramCompiled=false`
means the full source-project qualification gate remains open, despite native
object compilation and a complete minimal file in this particular probe.
Default KBase flags are 0x08000000 with zero FB/stack allocation fields, unlike
the saved source projects' zero flags and initialized FB/stack memory fields.
Those defaults must not become an implicit production image profile.
The independent backend still does not generate the global header or claim
storage initialization, linking of complete project images or hardware acceptance.

Ghidra exports 15/16 pin GenerateInst/AddToProgram/GetProgramBinary and header
option APIs. Copy prohibition is KBase+0x2ac, overwrite +0x2b0, extended
protection +0x2bc. Fields +0x29c/+0x2a0/+0x2a4 are still unresolved and must
not be assigned those names. ConvertPasswordString reverses bytes in each
four-byte group for an eight-byte buffer; it is not a password authenticator.
No real passwords or protected projects were involved.
