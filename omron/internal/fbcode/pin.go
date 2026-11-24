package fbcode

import (
	"strings"

	"github.com/joyautomation/nautilus/omron/cxp"
)

// ScalarTransferForPin connects a source-plan pin to the measured scalar copy
// stage. It derives types from PH_TYPE, never from the cached TypeName. It does
// not resolve a graph, infer caller state or perform complete project checking.
func ScalarTransferForPin(selector uint32, pin cxp.FBCallPin) ([]uint32, error) {
	d := pin.Declaration
	if d.Name == "" || (d.Usage != 1 && d.Usage != 2) || d.RemainingColumns[0] != "" || d.RemainingColumns[1] != "0" || d.RemainingColumns[3] != "0" || pin.Value.Source == "" {
		return nil, ErrPostamble
	}
	var kind ScalarType
	switch cxp.FBTypeName(d.TypeCode) {
	case "BOOL":
		kind = Bool
	case "INT":
		kind = Int
	case "UINT":
		kind = Uint
	case "WORD":
		kind = Word
	default:
		return nil, ErrPostamble
	}
	if symbol := pin.Value.Symbol; symbol != nil {
		if symbol.Name != pin.Value.Source || symbol.Address != pin.Value.Operand || symbol.Flags != "" || symbol.AuxType != "" || strings.ToUpper(symbol.DataType) != string(kind) {
			return nil, ErrPostamble
		}
	} else if pin.Value.Source != pin.Value.Operand {
		return nil, ErrPostamble
	}
	return ScalarTransfer(selector, kind, ParameterUsage(d.Usage), pin.Value.Operand)
}
