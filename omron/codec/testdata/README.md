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
