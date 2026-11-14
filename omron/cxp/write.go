package cxp

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// MarshalCXT emits the complete ordered tree, including unknown fields and
// graphical payloads. It keeps the chosen codepage; it does not regenerate
// CPU configuration or compile any instruction. Formatting is canonical.
func (d *Document) MarshalCXT() ([]byte, error) {
	return d.marshalCXT(MaxSize)
}

func (d *Document) marshalCXT(limit int) ([]byte, error) {
	var b strings.Builder
	var write func([]Field, int) error
	write = func(fields []Field, depth int) error {
		if depth > 64 {
			return errors.New("CXT nesting limit")
		}
		indent := strings.Repeat(" ", depth)
		for _, f := range fields {
			if strings.ContainsAny(f.Name, "\r\n:;") || f.Name == "" {
				return errors.New("invalid CXT field name")
			}
			b.WriteString(indent + f.Name + ":=")
			switch f.Kind {
			case "scalar":
				q := false
				done, err := scalarLine(f.Value+";", &q)
				if err != nil || !done {
					return fmt.Errorf("invalid scalar %s", f.Name)
				}
				b.WriteString(f.Value + ";\n")
			case "object":
				b.WriteString("\n" + indent + "BEGIN\n")
				if err := write(f.Fields, depth+1); err != nil {
					return err
				}
				b.WriteString(indent + "END;\n")
			case "raw", "list":
				end := f.Delimiter
				if f.Kind == "list" {
					if !strings.HasPrefix(end, "BEGIN_LIST_$#[") || !strings.HasSuffix(end, "]") {
						return errors.New("invalid list delimiter")
					}
					end = strings.Replace(end, "BEGIN_LIST_", "END_LIST_", 1) + ";"
				} else if !strings.HasPrefix(end, "$?St$Bk?_#[") || !strings.HasSuffix(end, "]") {
					return errors.New("invalid raw delimiter")
				}
				for _, line := range strings.Split(f.Raw, "\n") {
					if strings.TrimSpace(line) == end {
						return fmt.Errorf("payload contains its closing delimiter: %s", f.Name)
					}
				}
				b.WriteString("\n" + f.Delimiter + "\n")
				if f.Raw != "" {
					b.WriteString(f.Raw + "\n")
				}
				b.WriteString(end + "\n")
			default:
				return fmt.Errorf("unknown CXT field kind %q", f.Kind)
			}
			if b.Len() > limit {
				return errors.New("CXT output exceeds size limit")
			}
		}
		return nil
	}
	if err := write(d.Fields, 0); err != nil {
		return nil, err
	}
	encoded, err := encodeText(strings.ReplaceAll(b.String(), "\n", "\r\n"), d.Encoding)
	if err != nil {
		return nil, err
	}
	if len(encoded) > limit {
		return nil, errors.New("CXT encoded output exceeds size limit")
	}
	return encoded, nil
}

func encodeText(text, encoding string) ([]byte, error) {
	if !utf8.ValidString(text) {
		return nil, errors.New("invalid UTF-8 in project tree")
	}
	if encoding == "utf-8" {
		return []byte(text), nil
	}
	var table []rune
	switch encoding {
	case "windows-1251":
		table = cp1251[:]
	case "windows-1252":
		table = cp1252[:]
	default:
		return nil, fmt.Errorf("unsupported output encoding %q", encoding)
	}
	reverse := make(map[rune]byte, 128)
	for i, r := range table {
		if r != 0xfffd {
			reverse[r] = byte(i + 128)
		}
	}
	var out []byte
	for _, r := range text {
		if r < 128 {
			out = append(out, byte(r))
			continue
		}
		c, ok := reverse[r]
		if !ok {
			return nil, fmt.Errorf("character %U is not representable in %s", r, encoding)
		}
		out = append(out, c)
	}
	return out, nil
}

// SetRung replaces one textual mnemonic payload. Graphical FB rungs require a
// different editor and are refused, never flattened or replaced implicitly.
func (d *Document) SetRung(path, source string) error {
	if len(source) > MaxSize || strings.ContainsRune(source, 0) || !utf8.ValidString(source) {
		return errors.New("invalid rung source size/content")
	}
	source = strings.TrimSuffix(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	var target *Field
	var find func(string, []Field)
	find = func(prefix string, fields []Field) {
		for i := range fields {
			f := &fields[i]
			p := f.Name
			if prefix != "" {
				p = prefix + "." + p
			}
			if p == path {
				target = f
			}
			find(p, f.Fields)
		}
	}
	find("", d.Fields)
	if target == nil || !strings.HasPrefix(target.Name, "R[") {
		return fmt.Errorf("textual rung not found: %s", path)
	}
	for i := range target.Fields {
		f := &target.Fields[i]
		if f.Name != "SL" {
			continue
		}
		if f.Kind == "raw" {
			for _, line := range strings.Split(source, "\n") {
				if strings.TrimSpace(line) == f.Delimiter {
					return errors.New("rung contains its raw-block delimiter")
				}
			}
			f.Raw = source
			return nil
		}
		if f.Kind == "scalar" {
			f.Value = `"` + strings.ReplaceAll(source, `"`, `""`) + `"`
			return nil
		}
		return errors.New("unsupported rung payload")
	}
	return errors.New("rung has graphical elements; no textual SL to replace")
}

// Pack writes a standards-compliant DCL stream with literal tokens. It is
// intentionally unoptimized: source interoperability matters more than archive
// size. This is CXP container encoding, not PLC executable compilation.
func Pack(text []byte) ([]byte, error) {
	if len(text) > MaxSize || (len(text)*9+7)/8+6 > MaxSize {
		return nil, errors.New("CXP output exceeds size limit")
	}
	out := []byte{0, 6}
	pos := 16
	bits := func(v, n int) {
		for i := 0; i < n; i++ {
			if pos/8 == len(out) {
				out = append(out, 0)
			}
			out[pos/8] |= byte((v>>i)&1) << uint(pos%8)
			pos++
		}
	}
	for _, c := range text {
		bits(0, 1)
		bits(int(c), 8)
	}
	bits(1, 1)
	// Derive the inverted canonical code of length symbol 15 (end marker).
	first, index := 0, 0
	found := false
	for n := 1; n < 14 && !found; n++ {
		for j := 0; j < lengthTable.count[n]; j++ {
			if lengthTable.symbols[index+j] == 15 {
				code := first + j
				for i := n - 1; i >= 0; i-- {
					bits(((code>>i)&1)^1, 1)
				}
				found = true
				break
			}
		}
		index += lengthTable.count[n]
		first = (first + lengthTable.count[n]) << 1
	}
	bits(255, 8)
	return out, nil
}
