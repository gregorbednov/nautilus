# Instruction-library oracle capsule

Captured on 2026-11-14 using the installed CX-Pdata.dll with SHA-256
`a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141`.
Frida 17.26.3 executes research-only calls in an owned disposable x86 process.
Production code never executes these scripts.
The repository's existing `**/testdata/** -text` Git attribute preserves these
artifact bytes and line endings on Linux and Windows checkouts.

`library-manifest.json` records all artifact sizes/hashes and the configuration:
programType=0, unitVersion=null, timerCounterMode=library-default,
fullProgramCompiled=false, hardwareQualified=false.

- `library-cases.json`: 215 input cases, SHA-256
  `166a8ddb286e957a98507f20a28a0604ebdd1873eb198c4269aad9309344ef58`.
- `library-events.jsonl.gz`: exact event stream, decoded SHA-256
  `3acc1304ceb0b6d06ed8d1275ddfbfea467f3d8beabdff52e38ead22caf9806b`.
- `library-run.js.gz`: exact injected script with input literals.
- `library-template.js`, `library-worker.ps1`: source snapshots of the harness.

The stream includes both CPU profiles, 430 cases and a final completion record
after native object destruction. Worker and injector stderr were empty. A prior
cold run's 172 common cases produced identical words and rejection results.
Raw local provenance is at
`E:\cx-proger\research\cj1-oracle\instruction-library-03`.

Reproduce on the research Windows host with:

```powershell
.\tools\omron-research\Invoke-CJ1InstructionOracle.ps1 -Cases .\omron\codec\testdata\library-cases.json -Output E:\cx-proger\research\cj1-oracle\instruction-library-new
```

Existing project-oracle anchors are `omron/um/testdata/cj1h-baseline.n1` and
`cj1m-input-bit.n1`. Those GUI-derived outputs independently confirm LD W0.00,
LD W0.15, OUT W0.01 and END containers. This capsule is an instruction-unit
oracle, not proof of complete source-program acceptance or execution on a PLC.

## MOV and timer-mode continuation

`mov-manifest.json` independently pins 185 cases per CPU (370 observations),
their exact scripts and transcript. Input SHA-256:
`0e0fd67a737c8b95b10c374e11fab4d55c542bc99c040a0bc80d1176ef3f1f58`.
The raw capture is `E:\cx-proger\research\cj1-oracle\mov-library-02`.
The first87 cases per CPU were also measured in a separate cold worker run.

`timer-{library-default,bcd,binary}-manifest.json` each pin 14 cases per CPU
and matching harness snapshots (84 observations across the three modes).
Mode selection uses reviewed normal library APIs and readback via both getters.
These snapshots postdate the MOV capsule and include the new mode option.
They qualify research configuration behavior, not a production timer codec.
Raw directories: `E:\cx-proger\research\cj1-oracle\timer-mode-01-{MODE}`.

All manifests retain unknown unit version and false full-program/hardware
qualification. Tests verify artifact hashes, exact injected literals, event
order, profile selection and completion before using any observed results.

`timer-matrix-{bcd,binary}-manifest.json` pin 210 inputs for each CPU and mode
(840 observations total), with the same artifact roles and configuration checks.
The common input SHA-256 is
`e7fab8b1e11299c5f2a6f4f53b442b7b7c26f7d7e0b805fcc057e48a650f6e47`.
Raw directories are `E:\cx-proger\research\cj1-oracle\timer-library-02-{MODE}`.
Cases cover all six spellings, numeric/T/C indices, index boundaries, literal
boundaries, invalid BCD digits, direct preset areas and indirect negative controls.

`branch-manifest.json` pins262 inputs per CPU (524 observations) and matching
script/template/worker snapshots. Raw captures are
`E:\cx-proger\research\cj1-oracle\branch-library-01` and `branch-library-02`;
all188 common results match across these separate workers. Tests cover all16 TR
slots, leading-zero/lowercase spellings, missing/out-of-range/malformed relays,
allowed LD/OUT access, rejected other Boolean access, and zero-operand branch
instructions. Modifiers and incorrect labels remain unsupported even where the
low-level library emits containers. No whole-program stack or interlock checks
were performed by this oracle.

`modifier-manifest.json` pins454 inputs per CPU (908 observations). Input SHA-256:
`cf38e95f6f31f3b65e9d1a39c8637a12d41596c8507a58c7cb4a3229cf01f05f`.
The complete raw captures are
`E:\cx-proger\research\cj1-oracle\edge-library-02` and `edge-library-03`;
all492 shared results match across these workers. `edge-library-01` is an
incomplete failed probe and is deliberately not an accepted capsule: two input
operand collections serialized as null. The launcher now rejects such inputs
before creating output or starting a worker; zero operands must be `[]`.
This corpus includes allowed differentiated contacts and SET/RSET, address
boundaries, read-only A bits, DM/TR exclusions, modified OUT controls and other
unqualified instruction families. Whole-program checks and execution remain open.
