# Native N1 oracle

`cj1h-baseline.n1` is CX-Programmer 9.6.1.1's compiled output for the user's
generated CJ1H-H CPU65 baseline project, captured locally on 2026-11-14.
The source was checked with Unit Ver.4.0 selected: 0 errors,79 warnings.
No physical controller was used.

- Source CXP SHA-256: `1fe7d850676d7d0fe19510ceb58c7fe9ab2afd9bee1510f22a5bb8047abbd677`.
- CX-Pdata.dll SHA-256: `a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141`.
- Binary:4592bytes, SHA-256 `8a88c6e726542537787cea1419ccc42a1d203a37efb2304218c2a4e1687ce4f3`.
- Runtime event log SHA-256: `6b5e5fd6dc6846cfc0afafc8c583be1ae8fa687f1ba9d6d96990d9b13f50acda`.

The observer saw the compiler enter/return successfully and read its output
path. That file was copied unchanged; a subsequent compile file had the same
SHA-256. Raw evidence is outside the repository at
`E:\cx-proger\research\cj1-oracle\cj1h-baseline-runtime-01`.

This validates inspection of the captured layout, not CPU support for every
possible N1 variant, instruction encoding, hardware execution or deployment.
Unknown register/FB data remains opaque and is retained with the whole image.

The CJ1M CPU21 baseline compiled on the same date produced the identical
4592-byte image (source SHA-256
`07b842a8a2a9830f4778ee1ef4ed07111b5b508328adb1fec9055c388903a7c7`).
Its complete runtime log SHA-256 is
`cb702ba6bf3e8098033cc5ec5673bc4ffe2eadf37880bb1c72a798fa9c0a5422`;
raw evidence is in `cj1m-baseline-runtime-6048`. CPU resolution was observed as
CJ1M/CPU21, selector8, family mask0x400; the CJ1H compile used selector8,
family mask0x80000. Identical output here does not imply identical CPU palettes.

`cj1m-input-bit.n1` changes only the first rung's input from W0.00 to W0.15.
Its source CXP SHA-256 is
`becb4b4c39cad9f8a336ecc61a91957923e69ee46c990effca9125b9eb49badf`,
binary SHA-256 `52d783d09159eb2be3a76aa56dcf899bfd7d313da727107bcabe0a416b28eeb5`.
The bounded observer captured the entire output after successful compilation,
with no capture errors. Raw event SHA-256 is
`cfb018bd8ee5ba45edf4100661ba68f46ea224bd74ebc243d499451e8f253d7d`.
It differs from baseline at exactly byte3283 (00 -> 0F), the first code
container of task256 (e205e000 -> e205e00f). This single comparison does not
qualify a general address encoder.

The optional FB-prefix inspector now identifies one BLINK definition (record
word334, length46, header23, code23) and two instances (records388/442, length54,
header10, code44). Definition and instance sections begin at330/380. Both saved
instance buffers match complete176-byte runtime GetProgramBinary events in the
hash-pinned existing CJ1H/CJ1M logs. Relative code remains opaque.

`fb-writer-ghidra.json.gz` preserves an actual Ghidra12.1.4 selected-function
export including CompileMultipleProgramsAndProduceBinaryFile121c28ef; it shows
the header sizes/counts and prefix serialization. `fb-postencode-ghidra.json.gz`
preserves CreateTemporariesAndDoAsicFix121dea55 and DoAsicFixAndReturns121de87c.
Compressed/decoded hashes and compiler provenance are checked by tests. These
exports aid research; their inferred types are not a recovered compiler ABI.
See [the format findings](../../../docs/design/omron-fb-format.md).

## Task-file writer evidence

`task-file-writer-evidence.json.gz` retains10 complete static functions from
CX-P.exe and CX-Pdata.dll with five source-report hashes. The writer gates
the global N1 header/directory on selectorffff and record emission on task IDs.
Conditional auxiliary/FB blocks remain distinct; a task-record slice is not
a qualified standalone task compiler file or task-service payload.
Compressed SHA-256: `bf40c7b2e125e0630ccf6609f00fbfbcf9e4c02cb19e6a002884945314e6e6c5`.
`ExtractN1TaskRecord` copies exact bytes from validated full N1 fixtures; it
performs no native execution, instruction re-encoding or PLC operation.
See [writer and extraction](../../../docs/design/omron-task-writer-and-record-extraction.md).
