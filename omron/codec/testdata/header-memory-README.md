# Configured FB/stack memory descriptor

The `header-memory-*` capsule pins12 native observations: six cases on CJ1M
CPU21 and CJ1H-H CPU65, explicit BCD, unknown unit version. The disposable
worker uses normal SetUseFunctionBlockFlag, SetBaseRegister and SetStackRegister
calls. Five banks are set explicitly per case. Stack state is reset through
SetStackRegister(AC0,64,1536) before each requested setting. Public getters
record names, numeric addresses, sizes and stack split; only the latter's
numeric fields are also read directly. No object fields are forged.

Cases cover default banks/AC0, D0, shifted T/C/DIFF/H banks with D100 stack,
AC100 with zero first size, D32760 with equal first/total sizes, and an invalid
81/80 split. All base setters return1. Stack firstSize>totalSize returns0 and
preserves the reset state. The other five cases return1. All ordinary END
tasks and all12 minimal N1 files compile successfully.

`codec.EncodeN1FBMemory` independently generates a17-word configured descriptor:
seven address/size pairs (banks5/6 unused) followed by stack start, second-part
start and total size. The latter start is first start+firstSize. Address bases
are T=e000, C=f000, DIFF=8000, H=d800, AC=b200, D=10000. Ghidra19 pins the
setters/getters and Ghidra20 pins CreateBaseAddress121cec67. The replay checks
all configured numeric getters against independent output. The default
descriptor also matches header[15..31] in the existing saved FB image.

Important emission condition: writer121c28ef fills header[15..31] only when
its FB-definition count iStack_a70 is nonzero. Merely enabling FB mode or
setting banks does not suffice. All12 files here contain an ordinary task and
no definition; replay verifies every word of each656-word file, including
17 zero memory fields. Configured descriptor generation and emission are
therefore deliberately separate. The changed descriptors have not yet been
qualified in controlled full files containing FB definitions.

The Go API conservatively bounds banks0/1 to T/C3072..4095, bank2 to DIFF0..8191
with zero size, banks3/4 to H512..1407/H1408..1535, and stack to AC0..1535 or
D0..32767, with region-contained extents and total<=4096. These are supported
implementation limits, not a reconstruction of every native setter allowance.
No instance overlap/allocation audit, storage initialization or hardware
acceptance follows from encoding a descriptor.

Source mapping remains open. Explicitly setting source-like D0 produces10000,
not the b200 observed in the saved compiler image's AC0 stack. Do not silently
substitute one for the other or infer size units from CXT BitSize/WordSize.
Investigate the CX source-to-KBase initialization path before wiring this into
complete CXP compilation. The explicit API uses native word-address units.

Raw successful run: research/cj1-oracle/header-memory-01 outside Git. Reproduce
with Invoke-CJ1InstructionOracle.ps1 -ProgramObjectProbe -TimerCounterMode bcd
-Cases header-memory-cases.json -Output NEW_DIRECTORY. Portable replay needs
only Go and the archived assets. Attempts02/03/04 to construct a separate minimal
FB definition returned Program::Compile0 and remain failed raw research outside
the capsule; their helper was removed. They do not qualify a definition frontend.

Additional correction from Ghidra19: automatic allocation setters/getters
12012b40/12012b20 access KBase224, not220. The writer's220/bit200 remains unresolved.
