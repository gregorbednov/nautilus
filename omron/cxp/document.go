package cxp

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/il"
)

// Field preserves order, duplicate keys, multiline payloads and nested objects.
// Value is the exact scalar spelling; Raw is a delimited payload. Neither is
// interpreted as CPU bytecode. Source bytes remain available on Document.
type Field struct {
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	Delimiter string  `json:"delimiter,omitempty"`
	Value     string  `json:"value,omitempty"`
	Raw       string  `json:"raw,omitempty"`
	Fields    []Field `json:"fields,omitempty"`
	Line      int     `json:"line"`
}
type Document struct {
	SHA256   string  `json:"sha256"`
	Format   string  `json:"format"`
	Encoding string  `json:"encoding"`
	Fields   []Field `json:"fields"`
	Source   []byte  `json:"-"`
}

// Read detects CXP's DCL header or CXT's textual header and parses its
// assignment tree. Source is lossless (including the original text encoding).
func Read(data []byte) (*Document, error) { return ReadEncoding(data, "auto") }

// ReadEncoding accepts auto (UTF-8 if valid, otherwise Windows-1251), utf-8,
// windows-1251 or windows-1252. Source always retains the original bytes.
func ReadEncoding(data []byte, encoding string) (*Document, error) {
	if len(data) > MaxSize {
		return nil, errors.New("project exceeds size limit")
	}
	sum := sha256.Sum256(data)
	format := "cxt"
	text := bytes.Clone(data)
	if len(data) >= 2 && data[0] <= 1 && data[1] >= 4 && data[1] <= 6 {
		format = "cxp"
		var err error
		text, err = Explode(data, MaxSize)
		if err != nil {
			return nil, fmt.Errorf("CXP: %w", err)
		}
	}
	header := bytes.TrimPrefix(bytes.TrimSpace(text), []byte{0xef, 0xbb, 0xbf})
	if !bytes.HasPrefix(header, []byte("CXProgVer:=")) {
		return nil, errors.New("not a CX-Programmer CXP/CXT project")
	}
	decoded, encoding, err := decodeText(text, encoding)
	if err != nil {
		return nil, err
	}
	p := parser{lines: strings.Split(strings.ReplaceAll(decoded, "\r\n", "\n"), "\n")}
	fields, err := p.object(0, false)
	if err != nil {
		return nil, err
	}
	return &Document{SHA256: hex.EncodeToString(sum[:]), Format: format, Encoding: encoding, Fields: fields, Source: text}, nil
}

type parser struct {
	lines []string
	i     int
}

func (p *parser) object(depth int, nested bool) ([]Field, error) {
	if depth > 64 {
		return nil, errors.New("CXT nesting limit")
	}
	fields := make([]Field, 0)
	for p.i < len(p.lines) {
		line := strings.TrimSpace(p.lines[p.i])
		num := p.i + 1
		p.i++
		if line == "" {
			continue
		}
		if line == "END;" {
			if !nested {
				return nil, fmt.Errorf("line %d: unexpected END", num)
			}
			return fields, nil
		}
		key, val, ok := strings.Cut(line, ":=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("line %d: expected assignment", num)
		}
		f := Field{Name: strings.TrimSpace(key), Kind: "scalar", Line: num}
		val = strings.TrimSpace(val)
		if val == "" {
			if p.i >= len(p.lines) {
				return nil, fmt.Errorf("line %d: missing value", num)
			}
			next := strings.TrimSpace(p.lines[p.i])
			p.i++
			if next == "BEGIN" {
				f.Kind = "object"
				var err error
				f.Fields, err = p.object(depth+1, true)
				if err != nil {
					return nil, err
				}
			} else if strings.HasPrefix(next, "BEGIN_LIST_$#[") && strings.HasSuffix(next, "]") {
				f.Kind, f.Delimiter = "list", next
				end := strings.Replace(next, "BEGIN_LIST_", "END_LIST_", 1) + ";"
				start := p.i
				for p.i < len(p.lines) && strings.TrimSpace(p.lines[p.i]) != end {
					p.i++
				}
				if p.i == len(p.lines) {
					return nil, fmt.Errorf("line %d: unterminated list %s", num, next)
				}
				f.Raw = strings.Join(p.lines[start:p.i], "\n")
				p.i++
			} else if strings.HasPrefix(next, "$?St$Bk?_#[") && strings.HasSuffix(next, "]") {
				f.Kind, f.Delimiter = "raw", next
				start := p.i
				for p.i < len(p.lines) && strings.TrimSpace(p.lines[p.i]) != next &&
					!(f.Name == "STBodyText" && strings.HasSuffix(p.lines[p.i], next)) {
					p.i++
				}
				if p.i == len(p.lines) {
					return nil, fmt.Errorf("line %d: unterminated raw block %s", num, next)
				}
				if f.Name == "STBodyText" {
					// CX writes the closing marker immediately after the ST text.
					p.lines[p.i] = strings.TrimSuffix(strings.TrimRight(p.lines[p.i], " \t"), next)
					f.Raw = strings.Join(p.lines[start:p.i+1], "\n")
				} else {
					f.Raw = strings.Join(p.lines[start:p.i], "\n")
				}
				p.i++
			} else {
				return nil, fmt.Errorf("line %d: expected BEGIN or raw block", num)
			}
		} else {
			// CXT strings may span physical lines. A semicolon only ends an
			// assignment outside a quoted string (which doubles embedded quotes).
			var value strings.Builder
			quoted := false
			for {
				value.WriteString(val)
				done, err := scalarLine(val, &quoted)
				if err != nil {
					return nil, fmt.Errorf("line %d: %w", num, err)
				}
				if done {
					break
				}
				if p.i >= len(p.lines) {
					return nil, fmt.Errorf("line %d: unterminated scalar", num)
				}
				value.WriteByte('\n')
				val = p.lines[p.i]
				p.i++
			}
			f.Value = strings.TrimSuffix(strings.TrimSpace(value.String()), ";")
		}
		fields = append(fields, f)
	}
	if nested {
		return nil, errors.New("CXT: missing END")
	}
	return fields, nil
}
func scalarLine(s string, quoted *bool) (bool, error) {
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			if *quoted && i+1 < len(s) && s[i+1] == '"' {
				i++
				continue
			}
			*quoted = !*quoted
		}
		if s[i] == ';' && !*quoted {
			if strings.TrimSpace(s[i+1:]) != "" {
				return false, errors.New("data after scalar terminator")
			}
			return true, nil
		}
	}
	return false, nil
}

// Text returns a quoted CXT scalar as text without changing byte encoding.
func (f Field) Text() string {
	s := f.Value
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return strings.ReplaceAll(s[1:len(s)-1], `""`, `"`)
	}
	return s
}
func (f Field) Get(name string) (Field, bool) {
	for _, v := range f.Fields {
		if v.Name == name {
			return v, true
		}
	}
	return Field{}, false
}

// Walk visits each field with a stable fully qualified path.
func (d *Document) Walk(fn func(path string, f Field)) {
	var walk func(string, []Field)
	walk = func(prefix string, fs []Field) {
		for _, f := range fs {
			p := f.Name
			if prefix != "" {
				p = prefix + "." + p
			}
			fn(p, f)
			walk(p, f.Fields)
		}
	}
	walk("", d.Fields)
}

// Instruction retains the native instruction text. No claim of executable
// encoding or local VM support is made by recognizing a mnemonic.
type Instruction struct {
	MinimumCJ1UnitVersion string          `json:"minimumCJ1UnitVersion,omitempty"`
	Line                  int             `json:"line"`
	Text                  string          `json:"text"`
	Parsed                *il.Instruction `json:"parsed,omitempty"`
	SyntaxError           *il.SyntaxError `json:"syntaxError,omitempty"`
}
type Rung struct {
	Path         string        `json:"path"`
	Line         int           `json:"-"`
	Comment      string        `json:"comment,omitempty"`
	Instructions []Instruction `json:"instructions"`
	Graphic      []Field       `json:"graphic,omitempty"`
}
type CPU struct {
	Path   string `json:"path"`
	Config string `json:"config"`
	Device string `json:"device"`
	Model  string `json:"model"`
	Host   string `json:"host,omitempty"`
}
type Summary struct {
	FBDeclarationFindings []FBDeclarationFinding `json:"fbDeclarationFindings,omitempty"`
	Format                string                 `json:"format"`
	SHA256                string                 `json:"sha256"`
	TextBytes             int                    `json:"textBytes"`
	CPUs                  []CPU                  `json:"cpus"`
	Rungs                 []Rung                 `json:"rungs"`
	Mnemonics             map[string]int         `json:"mnemonics"`
	Warnings              []string               `json:"warnings"`
}

func (d *Document) Summary() Summary {
	s := Summary{Format: d.Format, SHA256: d.SHA256, TextBytes: len(d.Source), CPUs: []CPU{}, Rungs: []Rung{}, Mnemonics: map[string]int{}, Warnings: []string{}}
	d.Walk(func(path string, f Field) {
		if strings.HasPrefix(f.Name, "FunctionBlockWrapper[") && fbDefinitionPath.MatchString(path+".FunctionBlock") {
			s.FBDeclarationFindings = append(s.FBDeclarationFindings, inspectFBDeclarations(path, f)...)
		}
		if f.Name == "PLC" {
			if cfg, ok := f.Get("Config"); ok {
				c := CPU{Path: path, Config: cfg.Text()}
				for _, part := range strings.Split(c.Config, ";") {
					for _, pair := range []struct {
						key string
						dst *string
					}{{"DEV:", &c.Device}, {"CPU:", &c.Model}, {"IPADDR:", &c.Host}} {
						if i := strings.Index(part, pair.key); i >= 0 {
							*pair.dst = part[i+len(pair.key):]
						}
					}
				}
				s.CPUs = append(s.CPUs, c)
			}
		}
		if strings.HasPrefix(f.Name, "R[") || strings.HasPrefix(f.Name, "FBR[") {
			var graphic []Field
			for _, e := range f.Fields {
				if strings.HasPrefix(e.Name, "[") {
					graphic = append(graphic, e)
				}
			}
			sl, ok := f.Get("SL")
			if !ok {
				if len(graphic) > 0 {
					c, _ := f.Get("Com")
					s.Rungs = append(s.Rungs, Rung{Path: path, Comment: c.Text(), Instructions: []Instruction{}, Graphic: graphic})
				}
				return
			}
			source := sl.Raw
			if source == "" {
				source = sl.Text()
			}
			if strings.TrimSpace(source) == "" {
				if len(graphic) > 0 {
					c, _ := f.Get("Com")
					s.Rungs = append(s.Rungs, Rung{Path: path, Comment: c.Text(), Instructions: []Instruction{}, Graphic: graphic})
				}
				return
			}
			rung := Rung{Path: path, Instructions: []Instruction{}, Graphic: graphic}
			if c, ok := f.Get("Com"); ok {
				rung.Comment = c.Text()
			}
			for i, line := range strings.Split(source, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				lineNumber := sl.Line + i
				if sl.Kind == "raw" {
					lineNumber += 2
				}
				instruction := Instruction{Line: lineNumber, Text: line}
				parsed, err := il.Parse(line)
				if err == nil {
					instruction.Parsed = &parsed
					instruction.MinimumCJ1UnitVersion = codec.MinimumCJ1UnitVersion(parsed)
				} else {
					var syntax *il.SyntaxError
					if errors.As(err, &syntax) {
						instruction.SyntaxError = syntax
					}
					s.Warnings = append(s.Warnings, fmt.Sprintf("%s line %d: mnemonic syntax: %v", path, lineNumber, err))
				}
				rung.Instructions = append(rung.Instructions, instruction)
				words := strings.Fields(line)
				s.Mnemonics[words[0]]++
			}
			s.Rungs = append(s.Rungs, rung)
		}
	})
	s.Warnings = append(s.Warnings, "Native executable encoding and CPU configuration have not been verified; this is source inspection, not a deployable artifact.")
	for _, c := range s.CPUs {
		// Preserve configuration disagreement instead of trusting the filename.
		root := strings.TrimSuffix(c.Path, ".PLC")
		d.Walk(func(path string, f Field) {
			if path == root+".PLC.Settings.Data" {
				words := strings.Fields(f.Raw)
				raw := make([]byte, 0, len(words))
				for _, w := range words {
					n, e := strconv.ParseUint(w, 16, 8)
					if e != nil {
						return
					}
					raw = append(raw, byte(n))
				}
				header := strings.TrimRight(string(bytes.SplitN(raw, []byte{0}, 2)[0]), " ")
				if header != "" && !strings.Contains(header, c.Device+" "+c.Model) {
					s.Warnings = append(s.Warnings, fmt.Sprintf("%s: Config identifies %s %s but PLC settings header is %q", c.Path, c.Device, c.Model, header))
				}
			}
		})
	}
	return s
}
