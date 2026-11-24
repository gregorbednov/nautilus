# Task file table evidence

`task-table-evidence.json.gz` retains17 complete static listings with seven
source report hashes. It traces the44-byte in-memory record array, temporary
SEC/PRG names, derived task suffixes and the record+4/+c arguments passed to
task services463/526. Whole-program download uses the distinct owner+78 path.
The records contain CString fields and are not a wire format. Temporary-path
creation must not be treated as pure memory-only computation. No native code
is executed by these tests and no hardware write is qualified.

Compressed SHA-256: `d7cb3d054d7f4588703278b0ef9a249e6c2d6f45f186c8c2491b0bd9dc0ab232`.
See [task table report](../../../docs/design/omron-task-file-table.md).
