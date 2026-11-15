# Invocation postamble operations (observed CJ1 selector8)

This internal package implements a code-generation stage, not a public native
instruction codec or an FB compiler. No CXP/LD compiler invokes it yet.

`InvocationPostamble` follows AddtoInvoke_Postamble1217170b: the verified FBEND
form becomes9000080e; other compiled postambles clear CII bit20000000 and set
CC bits023e0000 only when the first word's c0000000 class equals80000000.
Operands and trailing words are unchanged. This is not a global rewrite rule.
Native SetCIIFlag requires compiled object flag80; the Go contract receives
already compiled words and does not emulate C++ object flags.

`InvocationSkeleton` reproduces the normal native preamble/epilogue lists from
GenerateInvocationCode12171499. Its boolean option is preserved literally:
true omits the PF0 save; its wider caller-state preconditions remain unknown.
The fixed recipes are OR CF115, optional BMOV PF0 ,IR62+, then BMOV ,-IR62 PF0
and FBEND around the body. `InvocationHeader` uses the generated instruction
count plus one, not the number of32-bit containers.

`AssembleInvocation` surrounds already encoded/allocated body instructions with
those recipes. It accepts measured transfer forms and exactly one FBICALL;
it does not validate private operand semantics, encode ports or allocate memory.
128 instructions is an implementation limit, not a qualified controller limit.
Failure returns no partial frame; successful output owns its word storage.

Evidence includes54 fresh primitive observations over both models, four actual
normal-object invocation-generation observations, Ghidra exports under testdata,
and all nine changes in the old full-project lifecycle trace. Frame assembly
matches all26 containers of the saved BLINK call. Full source-to-FB generation
remains unfinished; the current source-plan bridge is described below.

The next stage adds `ScalarTransfer`: BOOL and WORD/UINT/INT, explicit input or
output, scalar count1 only. A new176-row native capsule calls the actual
AddOperandPreamble121749ea on normally constructed objects. StringToDataType
returns1/2/4/8 for BOOL/INT/UINT/WORD; these are not PouVarList numeric codes.
WORD transfers use MOV with IR63 push/pop, BOOL uses BMOV with IR62 push/pop.
The generated child is typed (native flag40000000), explaining b241 rather than
the ordinary MOV b240 opcode before invocation postamble processing.

The Go operation returns the initial typed instruction for `AssembleInvocation`.
It shares the public codec's16-bit literal/direct-word rules. Qualified areas
are CIO/W/H/D for words and explicit CIO/W/H bits for BOOL; arrays, INOUT,
implicit ports, BOOL constants, private addresses and wider types fail closed.
This low-level operation receives typed source operands; source declarations
are connected through `ScalarTransferForPin` below. Allocation and general
graph compilation remain unimplemented.

The next source-plan bridge test reads `cxp.PlanFBCall` and verifies the complete
27-container LD+call against both captures. Port selection/order, bindings and
identifiers now come from CXP. The FBICALL recipe and invocation option remain
explicit test controls. No production source compiler is enabled.

PH_TYPE identity is now confirmed through PHeadDoc's x86 type library and the
actual CX-P GetItem3/constructor/row-writer chain. `ScalarTransferForPin` maps
BOOL0, INT1, UINT2 and WORD7 into the measured transfer forms. It validates
scalar metadata and exact symbol identity/address/type; it ignores cached
TypeName. All176 native transfer observations also exercise this pin bridge.
The complete-call comparison no longer has a test-only transport-type map;
FBICALL and invocation-option choice remain explicit controls. Complete project
typing, assignment-range checks, definition code and allocation remain pending.

See [the complete FB research](../../../docs/design/omron-fb-format.md).
