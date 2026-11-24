# Native arithmetic and UM service observations

The manifest pins two installed arithmetic modules and a100-byte standalone
function with no calls/globals: CS2 `subcb_sum_make`10009580 and plain10009750
have identical code SHA-256. The reviewed bytes ran in an owned x86 process on
own bounded buffers; no vendor DLL was loaded and no network was used. Neither
FINS handlers nor finalization routines were executed.

The worker checked76 deterministic synthetic cases (byte,16-bit and32-bit
modes, negative/zero counts, incomplete elements) plus four retained UM bodies.
All80 native outputs match. Go replay compares its32-bit accumulator with38
recorded synthetic outputs after explicit LE→BE representation conversion.
The remaining four observations retain only original image hashes, lengths,
service words and the native sum; full UM bytes remain outside Git.

The Ghidra asset retains selected assembly/pseudocode with module hashes and
engine version. It shows the virtual CPU's sum check,0307 final branch and
header writes. Its256-byte resident block quantum is not silently equated
with the64-byte relation found in four real-device images. Simulator-to-N1
layout mapping, actual CPU finalization, padding and transfer remain unqualified.

These assets qualify arithmetic and observed relations, not a header-writing
API or hardware deployment. The API's nativeAlgorithmConfirmed/writeQualified
remain false for the complete target-format/preparation question.

Full cases, pinned worker, extraction/export scripts and original Ghidra files:
workspace `research/hardware/20261116-um-native-sums-01` outside Git. Copy with
the raw hardware session directories when migrating. Reproduction needs the
exact installed binaries; replay here requires only Go and these assets.
