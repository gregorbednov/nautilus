# Read-only task-code observation

`cj1h-counter-code.json` retains the24 exact32-bit code words of task256 from
a2716-byte CJ1H readback on2026-11-16, full-image SHA-256 recorded in the JSON.
The same image was read repeatedly in the preceding inventory and the later
user-guaranteed stable window. Raw full images and responses remain outside Git
in workspace `research/hardware/20261116-cj1h-new4-inventory-01` and
`20261116-cj1h-stable-decode-01`.

The instruction spellings are the existing qualified Go codec's interpretation,
checked by exact re-encoding of every span. This is not an uploaded source/CXT,
vendor decompiler output, symbol table, or execution qualification. BCD/CJ1H-H
CPU65 is the explicit decode profile. The CMP #DEAD #DEAD sequence is retained
as a real instruction; its original editor/section role is not inferred.

Tests place the observed code into a synthetic Probe envelope derived from the
archived minimal no-FB native file. That envelope is not presented as a complete
live image. They check offsets, exact body coverage, opaque-tail preservation,
the report bound, profile handling, and independence from mutable diagnostics.
