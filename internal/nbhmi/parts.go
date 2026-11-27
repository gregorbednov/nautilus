package nbhmi

import (
	"fmt"
	"strconv"
	"strings"
)

func windowBounds(e *element) bounds {
	return bounds{number(e.value("Left")), number(e.value("Top")), number(e.value("Width")), number(e.value("Height"))}
}

func partBounds(e *element) bounds {
	x, y := number(e.value("Left")), number(e.value("Top"))
	return bounds{x, y, number(e.value("Right")) - x + 1, number(e.value("Bottom")) - y + 1}
}

func (c *converter) parts(parent *element, parts *[]part) error {
	if parent == nil {
		return nil
	}
	for _, e := range parent.Children {
		switch e.XMLName.Local {
		case "Part":
			p, err := c.component(e)
			if err != nil {
				return fmt.Errorf("%s %q: %w", e.attr("Type"), e.attr("Name"), err)
			}
			*parts = append(*parts, p)
		case "Group":
			if err := c.parts(e.child("GParts"), parts); err != nil {
				return err
			}
		case "draw":
			d, err := c.drawing(".", e)
			if err != nil {
				return fmt.Errorf("drawing %q: %w", e.attr("name"), err)
			}
			*parts = append(*parts, part{Kind: "draw", Name: e.attr("name"), Drawing: &d})
		default:
			return fmt.Errorf("unsupported parts element %q", e.XMLName.Local)
		}
	}
	return nil
}

func (c *converter) component(e *element) (part, error) {
	ctrl := e.child("CtrlEx")
	p := part{
		Kind: e.attr("Type"), Name: e.attr("Name"), Bounds: partBounds(e.child("Position")),
		Mode: number(e.value("Mode")), Interval: number(e.value("Interval")),
		Function:  e.attr("Function"),
		Confirm:   ctrl.attr("bOPConfirm") == "1",
		AutoLogin: ctrl.attr("AutoPWD") == "1",
	}
	if raw := ctrl.attr("WaitTime"); p.Confirm && raw != "" {
		wait, err := strconv.Atoi(raw)
		if err != nil {
			return p, fmt.Errorf("invalid operation confirmation timeout %q", raw)
		}
		p.ConfirmWaitSeconds = &wait
	}
	switch p.Kind {
	case "Bit Lamp", "Word Lamp", "Switch", "Set Bit", "Word Set", "Number Input",
		"Text Input", "Text Display", "Function Key", "Direct Window", "FileList", "Vector graph":
	default:
		return p, fmt.Errorf("unsupported component kind")
	}
	if e.child("Position") == nil || p.Bounds.W <= 0 || p.Bounds.H <= 0 {
		return p, fmt.Errorf("invalid geometry")
	}
	if ctrl.attr("bMacro") == "1" || e.child("sound").attr("bUSeSound") == "1" {
		return p, fmt.Errorf("active macro or sound")
	}
	if w := e.value("WindowID"); w != "" {
		id, err := strconv.Atoi(w)
		if err != nil {
			return p, fmt.Errorf("invalid WindowID")
		}
		p.Target = &id
	}
	if kb := e.child("Keyboard"); kb != nil && kb.attr("bNoUseKB") != "1" && kb.attr("Window") != "" {
		id, err := strconv.Atoi(kb.attr("Window"))
		if err != nil {
			return p, fmt.Errorf("invalid keyboard window")
		}
		p.Keyboard = &id
	}
	switch p.Kind {
	case "Function Key":
		if p.Function != "Change Frame" && p.Function != "Input char" {
			return p, fmt.Errorf("unsupported key function %q", p.Function)
		}
		if p.Function == "Change Frame" {
			mode := e.attr("Mode")
			switch mode {
			case "Change window", "Close window", "Popup window title bar":
				p.Meta = map[string]string{"navigation": mode}
			default:
				return p, fmt.Errorf("unsupported window key mode %q", mode)
			}
		}
		if char := e.child("Char"); char != nil {
			p.Char = char.Text
		}
	case "Switch":
		if p.Mode < 0 || p.Mode > 2 {
			return p, fmt.Errorf("unsupported switch mode %d", p.Mode)
		}
	case "Set Bit":
		if p.Mode < 0 || (p.Mode > 2 && p.Mode != 6 && p.Mode != 7) {
			return p, fmt.Errorf("unsupported bit button mode %d", p.Mode)
		}
	case "Word Lamp":
		p.Meta = make(map[string]string)
		for _, value := range e.child("Map").children("MapValue") {
			p.Meta[strings.TrimSpace(value.Text)] = value.attr("ID")
		}
	case "FileList":
		p.Meta = map[string]string{"suffix": e.child("Suffix").attr("SuffixList")}
	case "Word Set":
		for _, key := range []string{"Param1", "Param2", "Param3", "Param4", "Param5", "Flag"} {
			p.Parameters = append(p.Parameters, number(e.value(key)))
		}
	case "Vector graph":
		p.Mode = number(e.value("VGState"))
	case "Number Input":
		data := e.child("DataSet")
		if data.attr("Convert") == "Yes" {
			return p, fmt.Errorf("unsupported numeric conversion")
		}
		format := data.attr("DataType")
		switch format {
		case "0", "1", "2", "3", "4":
		default:
			return p, fmt.Errorf("unsupported numeric data type %q", format)
		}
		minimum, minErr := strconv.Atoi(data.value("MinValue"))
		maximum, maxErr := strconv.Atoi(data.value("MaxValue"))
		decimals, decimalErr := strconv.Atoi(data.value("DecimalCount"))
		if minErr != nil || maxErr != nil || decimalErr != nil || minimum > maximum || decimals < 0 || decimals > 9 {
			return p, fmt.Errorf("invalid numeric limits or decimal count")
		}
		if format != "0" && format != "1" {
			decimals = 0
		}
		p.Parameters = []int{minimum, maximum, decimals}
		p.Meta = map[string]string{"numberFormat": format}
	case "Text Input", "Text Display":
		p.Meta = map[string]string{"alignment": e.child("TextPosition").attr("Align")}
	}
	if shape := e.child("Shape"); shape != nil && shape.attr("URL") != "" {
		states, err := c.shape(shape.attr("URL"))
		if err != nil {
			return p, err
		}
		p.Assets = states
	}
	if e.child("Lable").attr("UseTextLibrary") == "Yes" {
		return p, fmt.Errorf("unsupported text library reference")
	}
	for _, s := range e.child("Lable").child("States").children("State") {
		font := s.child("Text").child("Font")
		if font != nil {
			p.Labels = append(p.Labels, fontLabel(font, font.Text))
		} else {
			p.Labels = append(p.Labels, label{})
		}
	}
	// NB switches store the two labels in the opposite order to their graphics.
	if p.Kind == "Switch" && len(p.Labels) == 2 {
		p.Labels[0], p.Labels[1] = p.Labels[1], p.Labels[0]
	}
	if box := e.child("Lable").child("Position"); box != nil {
		b := partBounds(box)
		p.LabelBox = &b
	}
	if len(p.Labels) == 0 && e.child("Font") != nil {
		p.Labels = []label{fontLabel(e.child("Font"), "")}
	}
	limit := 0
	for _, a := range e.children("Address") {
		function := a.attr("Function")
		if function == "Limit" && e.attr("LimitSwitch") == "Off" {
			continue
		}
		r, err := c.address(a)
		if err != nil {
			return p, err
		}
		switch function {
		case "Read":
			p.Read = r
		case "Write":
			p.Write = r
		case "Limit":
			if limit == 0 {
				p.High = r
			} else if limit == 1 {
				p.Low = r
			}
			limit++
		case "Note":
			// Text annotation binding used by the original file dialog.
		default:
			return p, fmt.Errorf("unsupported address function %q", function)
		}
	}
	if p.Kind == "Set Bit" && p.Write == nil {
		p.Write = p.Read
	}
	if p.Kind == "Set Bit" && p.Mode > 2 && (p.Write == nil || p.Write.Area != "LB") {
		return p, fmt.Errorf("window lifecycle may only change local bits")
	}
	var err error
	if p.Visible, err = c.condition(e.child("View")); err != nil {
		return p, err
	}
	if p.Enabled, err = c.condition(e.child("Enabled")); err != nil {
		return p, err
	}
	if notify := e.child("CtrlEx"); notify.attr("bNotify") == "1" {
		r, err := c.address(notify.child("Address"))
		if err != nil {
			return p, fmt.Errorf("operation notification: %w", err)
		}
		if r != nil {
			if r.Area == "CIO" {
				return p, fmt.Errorf("unsupported notification to a PLC address")
			}
			if p.Meta == nil {
				p.Meta = make(map[string]string)
			}
			p.Meta["notify"] = r.Key
			p.Meta["notifyValue"] = notify.attr("NotifyVal")
		}
	}
	return p, nil
}

func (c *converter) condition(e *element) (*condition, error) {
	if e.attr("CtrlMode") == "" || e.attr("CtrlMode") == "0" {
		return nil, nil
	}
	cond := &condition{
		On:    e.attr("bCondBitOn") == "1",
		Login: e.attr("bCondUser") == "1" || e.attr("bCondAuthority") == "1",
	}
	if e.attr("bCondReg") == "1" {
		r, err := c.address(e.child("Address"))
		if err != nil {
			return nil, err
		}
		if r == nil || e.attr("bCondWord") == "1" {
			return nil, fmt.Errorf("unsupported word or missing register condition")
		}
		cond.Register = r
	}
	return cond, nil
}

func (c *converter) address(e *element) (*register, error) {
	addr := e.child("RegAddr")
	if addr == nil {
		return nil, nil
	}
	if e.attr("UseIndexAddr") == "1" || e.attr("ImportTag") == "Yes" {
		return nil, fmt.Errorf("unsupported indirect or symbolic address")
	}
	value, area := addr.value("AddressValue"), addr.value("AddressType")
	count := max(1, number(addr.attr("WordCount")))
	switch area {
	case "LB", "LW":
		index, err := strconv.Atoi(value)
		if err != nil || index < 0 || index > 65535 {
			return nil, fmt.Errorf("invalid %s address %q", area, value)
		}
		return &register{fmt.Sprintf("%s%d", area, index), area, index, count}, nil
	case "CIO_bit":
		wordText, bitText, ok := strings.Cut(value, ".")
		word, wordErr := strconv.Atoi(wordText)
		bit, bitErr := strconv.Atoi(bitText)
		if !ok || wordErr != nil || bitErr != nil || word < 0 || word > 6143 || bit < 0 || bit > 15 ||
			c.plcID == "" || addr.value("PLCID") != c.plcID || count != 1 {
			return nil, fmt.Errorf("invalid or unsupported CIO address %q for PLC %q", value, addr.value("PLCID"))
		}
		key := cioBitTag(word, bit)
		c.tags[key] = fmt.Sprintf("CIO%d.%02d", word, bit)
		return &register{key, "CIO", word*16 + bit, 1}, nil
	default:
		return nil, fmt.Errorf("unsupported address area %q", area)
	}
}

func cioBitTag(word, bit int) string {
	return fmt.Sprintf("NB_CIO_%04d_%02d", word, bit)
}

func fontLabel(e *element, value string) label {
	size := number(e.attr("VectorFontSize"))
	if size <= 0 {
		size = number(e.attr("Size"))
	}
	fill := e.attr("VectorFontColor")
	if fill == "" {
		fill = e.attr("Color")
	}
	return label{
		Text: textLines(value), Size: max(size, 12), Color: color(fill),
		Align: number(e.attr("VectorFontAlign")), Bold: e.attr("VectorFontBold") == "1",
	}
}

func textLines(s string) string {
	return strings.NewReplacer("$_enter_$", "\n", "$BK_RETURN$", "\n").Replace(s)
}
