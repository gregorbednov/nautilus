# Portable CJ1 oracle verification

This research tool validates saved evidence on Linux or Windows. It neither
launches CX-One nor connects to a PLC. Production Omron packages do not import
it. Run from the repository root:

```sh
go run ./tools/omron-research/oracle verify -root .
go build -o bin/cj1-oracle ./tools/omron-research/oracle
bin/cj1-oracle audit-capture -log /path/events.log -stderr /path/stderr.log
```

On Windows build `bin/cj1-oracle.exe` before using `Capture-CJ1Oracle.ps1`.
The launcher runs the auditor before extracting accepted binary files and
preserves its report in `capture-audit.json`. An incomplete capture fails the
command while retaining raw observations for investigation.

The portable manifest `testdata/manifest.json` reconstructs each generated
source from an original CXP fixture plus an exact rung replacement. It checks
both generated CXT and CXP hashes. This avoids duplicating whole projects for
each source variant. Gzipped event transcripts retain their exact decoded hash,
including original line endings; binary files remain in `omron/um/testdata`.
Artifact paths are repository-relative, bounded and cannot escape via symlinks.

The auditor checks sequence continuity, process identity, module provenance
assertions, lifecycle, stderr, dropped events, compiler entry/return, successful
file capture and its hash. New observer events carry `compileSequence` for
explicit operation correlation. Original transcripts retain their weaker
thread-order correlation. Default initialization profiles are kept separate
from the profile selected by the actual compile call.

An internally consistent transcript does not authenticate its producer.
Reproducing a source recipe also does not establish which project was loaded
in the compiler process. All current source identities remain operator-reported;
hardware unit versions are unknown and hardware qualification remains false.

The early CJ1H capture used an external file copy and lacks its exact observer
snapshot. The manifest explicitly labels it `operator-copy`: it passes artifact
and profile consistency checks but reports `captureComplete=false` and
`binaryBindingVerified=false`. It is never silently promoted to the stronger
`embedded-file` evidence available for the two CJ1M captures.

Tests replay real transcripts and reject mismatched source recipes, image
hashes, CPU profiles, comparisons, failed/orphaned operations, corrupt data,
missing lifecycle events, stderr failures and unsupported qualification claims.
The manifest verifier accepts only the current evidence schema, not an arbitrary
CPU instruction catalog or a deployable-program certification.
