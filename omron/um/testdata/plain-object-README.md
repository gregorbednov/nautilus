# Plain CJ1 object container evidence

`plain-object-writer-ghidra.json.gz` retains the complete static listing and
pseudocode for CX-P.exe12685840, source SHA-256
`e91f15b8613e56c8f326bc83ce07f5a24b177f5f70a4a445e4eadfa34b0a9eb4`.
Compressed SHA-256:
`f6b006cfbf18295771c1f080a6cd82f6e1fb87b65d87a9f092b91d59f94e6e88`.

The plain subset writes48 zero bytes and copies the complete source file.
The protected branch writes marker80/81 and an additional380-byte block.
Native code was not executed; no passwords or protected project data were read.
Tests construct plain containers using the two existing compiler N1 fixtures;
there is no captured native OBJ golden file yet. This is local-file evidence,
not hardware/write qualification. See
[full report](../../../docs/design/omron-plain-object-envelope.md).
