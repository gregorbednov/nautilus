# CJ1 connection data API and hardware observations - 2026-11-19

`fins.Client.ReadConnections(ctx)` now implements read-only CONNECTION DATA
READ0502 independently of CX-One. Two complete enumerations on each real CPU
returned identical four-record results. Raw CPU identity/status stayed equal
before/after: RUN4, fatal0, nonFatal0.

## CPU-reported devices

All addresses below are hexadecimal FINS unit addresses, not physical slots.

| Address | CJ1H10.0.174.37 | CJ1M10.0.174.36 |
| --- | --- | --- |
| 00 | CJ1H_CPU65H | CJ1M_CPU21 |
| 11 | CJ1W-ETN21 | CJ1W-ETN21 |
| 12 | CJ1W-DRM21 | CJ1W-SCU41-V1 |
| 23 | CJ1W-MAD42 | no record at this address |
| 29 | no record at this address | CJ1W-MAD42 |

Address11 corresponds to CPU Bus unit number1,12 to CPU Bus unit number2.
The Special I/O MAD42 address is20+unit number:3 on CJ1H and9 on CJ1M.
The CJ1H list matches the earlier16 November0502 observation. The CJ1M list
provides a new live comparison point, without treating saved CXP names as
controller observations. This command does not enumerate all Basic I/O modules
or establish physical rack/slot/terminal wiring. No full physical inventory is
claimed.

## Implemented contract

Primary source: local `research/W342.pdf`, section5-3-16, printed pages193-194
(PDF215-216); diagrams were checked visually and retained with the evidence.
Requests contain an explicit starting address and1..25 record count. The
response DATA begins with count (bit7 marks the final unit), followed by
count records of one address byte and20 ASCII model bytes.

The API starts at CPU00 and paginates forward through the documented addresses:
00,10..1F CPU Bus,20..7F Special I/O, E1 Inner Board. Missing addresses may be
skipped by the CPU. The next request begins after the last returned address,
skipping undocumented address gaps. Transport response limits further bound
the requested count. Returned records must be ordered, nonduplicated, at or
after the requested start, and have exact lengths and nonempty printable model
names. The first record must be the CPU. An unterminated enumeration atE1 is
rejected. This policy guarantees bounded forward progress without arbitrary
address scanning.

The API returns `[]ConnectedUnit` (Address, Model, privately copied20-byte
RawModel). Malformed payloads terminate the client through shared validation;
any error returns nil rather than a partial inventory. There is no retry,
reconnection, mode change, access acquisition or write. Valid PLC end errors
retain the existing typed read-only handling. Multiple pages are not atomic;
callers should exclude configuration changes and compare repeat enumerations.

## Actual read-only trial

CJ1H capture:05:32:09-05:32:10 UTC (08:32:09-08:32:10 Moscow).
CJ1M capture:05:32:10-05:32:11 UTC (08:32:10-08:32:11 Moscow).
Each CPU received0501/0601, two0502 requests `00 19`, then0501/0601.
Every0502 reply had count/last84 and85 DATA bytes. Complete DATA SHA256:

- CJ1H:`49259418e034292199f86fcd6e7a12c6ad16f0f99f0c22a98dccc7b4e57b57c8`.
- CJ1M:`484c133ceb06e3d28dd6db22ef191efe478ab1d3e62905c6cce5918cd296e3c7`.

A transparent loopback recording relay forwarded production DialTCP to the
fixed PLC endpoint. `backendBinding.peer` is the relay; `relayMapsTo` records
the actual endpoint. Negotiated source node239/destination37 or36 and all
backend response checks still apply; these are metadata, not authentication.
An independent Python verifier reconstructed raw TCP frames, checked complete
handshake/envelope/route/SID/command/end-code consistency, and compared both
model lists and raw CPU/status to the saved JSON. Exactly12 requests occurred:
four0501, four0601, four0502. No control, program-write or parameter-write
commands were present. Program memory and parameter areas were not refreshed
in this trial; use their separately timestamped captures.

Frozen evidence: `F:/cx-proger/research/hardware/20261119-cj1-connections-01`.
25 files excluding SHA256SUMS; manifest SHA256:
`5aeb6167c078ba1708934073007cd97de2dac0a067b2b64d781e78ad712ecc61`.
Includes production source snapshots, driver, raw request/response streams,
connection pairs, CPU/status, manual extracts/images, independent verifier,
provenance, fixture and validation record. Existing frozen parameter evidence
was not changed.

## Validation and remaining gates

All10 Omron packages passed; the final FINS package passed again after adding
the two retained hardware fixtures, and go vet passed for all Omron packages.
Windows amd64, Go1.26.10, CGO_ENABLED=0, offline dependencies. Local tests cover
all114 documented addresses, sparse lists, three transport capacities,
malformed count/length/address/model/order/termination, later-page PLC error,
undersized transport, owned RawModel bytes and replay of both actual replies.
The only hardware pagination observed here was a single final page; multi-page
behavior remains covered locally rather than qualified on a populated rack.

This closes the typed read-only connection-data gap and confirms both
controllers' CPU Bus/Special I/O model lists. It does not select0307/0317,
resolve IsExtendedProtect or CPU finalization, decode a complete physical I/O
table, or qualify program transfer/restoration. Next work remains target
protection/service selection and the concrete program-write/original-return
trial. Zero-filled PLC Setup from the preceding capture is insufficient to
infer those properties. WriteQualified/RestoreQualified remain false.
