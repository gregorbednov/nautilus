package cxp

// FBTypeName identifies PH_TYPE values from the PHeadDoc type library. These
// ordinal declaration codes are distinct from KBaseComp's nDATA_TYPE bitmask.
// Knowing a name does not qualify its operands, allocation or native lowering.
func FBTypeName(code int) string {
	names := [...]string{
		"BOOL", "INT", "UINT", "DINT", "UDINT", "LINT", "ULINT", "WORD",
		"DWORD", "LWORD", "REAL", "LREAL", "COUNTER", "TIMER",
		"FUNCTION_BLOCK", "STRING", "STRUCTURE",
	}
	if code < 0 || code >= len(names) {
		return ""
	}
	return names[code]
}
