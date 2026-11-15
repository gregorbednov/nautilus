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
and all nine changes in the old full-project lifecycle trace. With the five
known initial encoder body controls, frame assembly matches all26 containers
of the saved BLINK call. That body is still supplied as encoded controls in the
test, not compiled from CXP. Full source-to-FB generation remains unfinished.

See [the complete FB research](../../../docs/design/omron-fb-format.md).
