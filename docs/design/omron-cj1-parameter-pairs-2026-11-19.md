# CJ1H/CJ1M parameter pairs - 2026-11-19

The requested read-only capture succeeded for all four separate parameter
areas on both real controllers. Each of eight pairs contains two complete,
byte-identical copies. CPU identity and full raw status remained identical
across all four area captures for each target: RUN4, fatal0, nonFatal0.

- CJ1H `10.0.174.37:9600`, CJ1H_CPU65H:05:24:26-05:24:31 UTC
  (08:24:26-08:24:31 Moscow).
- CJ1M `10.0.174.36:9600`, CJ1M_CPU21:05:24:31-05:24:36 UTC
  (08:24:31-08:24:36 Moscow).

## Saved copies

Byte counts are per copy; SHA256 is shared by copy1 and copy2 of that area.

| Target | Area/code | Bytes | SHA256 |
| --- | --- | ---: | --- |
| cj1h | plc-setup/8010 | 1024 | `5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef` |
| cj1h | io-table/8012 | 2560 | `63ebaaf0b8a442875b7880eae77f0d892f41c3e897b2d7f80489868546c6a786` |
| cj1h | routing/8013 | 1024 | `5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef` |
| cj1h | cpu-bus/8002 | 10368 | `4ab7065ca0c414aedf244961aa00d7b3be7518eee5e1d8b9fb14809f793ab26a` |
| cj1m | plc-setup/8010 | 1024 | `5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef` |
| cj1m | io-table/8012 | 2560 | `f3ac9577db4093e5275b7ddff43d62cdb7b2f60b7bdf1baa7f24bc570510bbda` |
| cj1m | routing/8013 | 1024 | `5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef` |
| cj1m | cpu-bus/8002 | 10368 | `b3060f35cf73ad33ff5ba841066b4ebf6e517580dc87ea167afcd7ddcde329c9` |

PLC Setup8010 and routing8013 are entirely zero-filled on both CPUs. These
are actual returned DATA bytes, not placeholder files: independent TCP
reassembly verified every block against the saved copies. I/O registration8012
and CPU Bus Setup8002 contain nonzero bytes and differ between the targets.
No meaning is assigned to individual configuration bits in this capture.

## Wire verification and provenance

The driver used production `fins.DialTCP`, `Identity`, `Status` and
`ReadParameterArea` at source commit76bdf3cc. A transparent loopback TCP relay
recorded both streams for each area session. `backendBinding.peer` is therefore
the relay listener, while `relayMapsTo` is the actual PLC endpoint; the relay
forwards to the fixed target. Node negotiation and all backend response
validation still ran. This records routing metadata, not device authentication.

Each area session read CPU/status, copy1, copy2, then CPU/status. All area reads
started at relative word0; chunks were at most497 words. The CPU returned
successful end codes, exact area/address/count echoes and the last-word bit
only at the documented boundary on both copies of all eight areas.

An independent offline Python verifier reconstructed each TCP envelope and
FINS exchange, checked node negotiation, route/SID/command correlation and
zero end codes, and reconstructed all16 files (59904 bytes) from response DATA.
It also compared raw CPU/status to JSON and across all areas for each target.
The recorded command totals are exactly:

- 72 parameter reads0201;
- 16 identity reads0501;
- 16 status reads0601.

There are no control/access/program-write/parameter-write commands in either
controller's recorded request streams. No CX-One runtime was used.

Frozen raw evidence: `F:/cx-proger/research/hardware/20261119-cj1-parameter-pairs-01`.
49 files are listed in SHA256SUMS (the manifest itself is not listed).
Manifest SHA256:
`cd4cb0ab1704d497c70f61966441f1080766f156c6b85bb58b04ee1640d8f54b`.
The package includes the driver, backend source snapshots,16 copy files,
per-area result JSON and two TCP streams, summary, provenance, offline verifier
and wire-verification report. Re-running `verify_capture.py` checks existing
files and report without opening a network connection or changing the report.

## What this closes and what remains

Read availability, documented separate-area sizes, response format and terminal
flags are now observed on CJ1H_CPU65H and CJ1M_CPU21. Two-copy consistency was
established during this short capture. The read API needs no format correction.
No backend code changed for this evidence stage; validation was the real
read-only capture, independent wire reconstruction and manifest verification.

The I/O table remains registered configuration, not independently observed
physical modules. Sequential blocks and separate area sessions are not an
atomic snapshot. These files supplement UM backups; they do not include all
DM/HR/EM values, files, comments/symbols or every unit's internal state.
No UM read was requested in this capture, so it does not refresh earlier UM
backups. FullControllerBackup/WriteQualified/RestoreQualified remain false.

Next: decode relevant setup/protection and unit data from primary/native
sources, compare registered modules with physical unit information, and settle
0307/0317 target service, profile/finalization and the concrete original-return
procedure before a first program-write trial. Zero-filled PLC Setup alone does
not resolve IsExtendedProtect or qualify a write.
