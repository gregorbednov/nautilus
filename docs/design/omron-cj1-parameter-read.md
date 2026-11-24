# CJ1 parameter-area read preparation - 2026-11-19

`fins.Client.ReadParameterArea(ctx, area)` reads an entire selected area with
FINS 0201. It issues no mode/access/write command and is not wired into a
production download path. The original implementation stage was local only;
[subsequent hardware capture](omron-cj1-parameter-pairs-2026-11-19.md) now
confirms matching full-area pairs and terminal flags on both real CPUs.

## Documented contract

Primary source: local `research/W342.pdf`, section 5-3-7, printed pages181-182
(PDF pages203-204). The area table was checked visually, not only by extraction.

| Go selector | Wire code | Words |
| --- | --- | --- |
| ParameterPLCSetup | 8010 | 512 |
| ParameterIOTable | 8012 | 1280 |
| ParameterRoutingTable | 8013 | 512 |
| ParameterCPUBusSetup | 8002 | 5184 |

Combined8000 and unknown areas are deliberately unsupported. Each read starts
at relative word0 and covers the entire documented separate area. The six-byte
request carries BE16 area/start/count, with count bit15 clear. Responses must
repeat area/start/count exactly, contain count*2 bytes, and set bit15 exactly
when the documented last word is returned. Data retain wire byte order.

W342 permits498 words per request. This implementation also budgets the
six-byte response prefix against transport MaxReadData, hence497 words with
the default1000-byte response limit. Small transports receive smaller chunks.
Over-capacity requests are not used to discover boundaries. End codes1104 and
1108 are errors, not usable partial backups or retry instructions.

Malformed successful responses close the client through shared validation.
Any error returns nil rather than accumulated partial bytes. A valid PLC end
code remains a typed EndError under the existing read-only client policy.
There is no fallback to another area, retry, write, or configuration change.

## Evidence limits and next step

Local tests check all four full ranges, three transport sizes, malformed
area/address/count/length/final flags, unsupported selectors, insufficient
transport capacity and PLC end errors both before and after a partial read.
Existing Omron tests continue to exercise shared framing and correlation.
These tests established the initial local protocol contract; the linked
subsequent report records the hardware-confirmed area dumps.

Sequential blocks are not an atomic snapshot. A future capture must save raw
CPU/status around two complete copies of every selected area, compare those
copies and retain per-area errors and hashes. Availability, ranges and final
flags have now been observed on both real CPUs in the linked capture. A rejected read must not
trigger clearing protection or changing settings.

The I/O table is **registered configuration**, not proof of physically mounted
modules. Parameter copies do not include all DM/HR/EM values, files, symbols,
comments or every unit's internal configuration. They do not provide a full
controller backup or a tested restoration procedure. Raw PLC Setup bytes alone
do not settle IsExtendedProtect, physical unit version,0307/0317 selection or
CPU finalization. These remain gates before the first program-write trial.

## Validation

All10 Omron packages passed `go test -buildvcs=false -mod=readonly -timeout
180s ./omron/...`. The final Parameter test subset passed separately after
adding the later-block error case; `go vet ./omron/fins` passed. Environment:
Windows amd64, Go1.26.10, CGO_ENABLED=0, dependencies from the offline cache.
