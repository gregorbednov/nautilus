package nbhmi

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

const maxSourceSize = 32 << 20

type element struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Children []*element `xml:",any"`
	Text     string     `xml:",chardata"`
}

func (e *element) attr(name string) string {
	if e != nil {
		for _, a := range e.Attrs {
			if a.Name.Local == name {
				return a.Value
			}
		}
	}
	return ""
}

func (e *element) child(name string) *element {
	if e != nil {
		for _, c := range e.Children {
			if c.XMLName.Local == name {
				return c
			}
		}
	}
	return nil
}

func (e *element) children(name string) []*element {
	var found []*element
	if e != nil {
		for _, c := range e.Children {
			if c.XMLName.Local == name {
				found = append(found, c)
			}
		}
	}
	return found
}

func (e *element) value(name string) string {
	if c := e.child(name); c != nil {
		return strings.TrimSpace(c.Text)
	}
	return ""
}

func number(s string) int {
	i, _ := strconv.Atoi(strings.TrimSpace(s))
	return i
}

func readXML(root *os.Root, name string) (*element, error) {
	f, err := root.Open(filepath.FromSlash(name))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() || info.Size() > maxSourceSize {
		return nil, fmt.Errorf("%s: expected a regular XML file up to %d bytes", name, maxSourceSize)
	}
	d := xml.NewDecoder(io.LimitReader(f, maxSourceSize+1))
	var e element
	if err := d.Decode(&e); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	for {
		t, err := d.Token()
		if err == io.EOF {
			return &e, nil
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		switch v := t.(type) {
		case xml.Comment, xml.ProcInst:
			continue
		case xml.CharData:
			if strings.TrimSpace(string(v)) == "" {
				continue
			}
		}
		return nil, fmt.Errorf("%s: unexpected content after root element", name)
	}
}

// resolveNB resolves a path relative to base with Windows-style separators.
// Old directories often have inconsistent filename casing after being copied
// to Linux. Prefer the exact name and reject an ambiguous case-folded match.
func resolveNB(root *os.Root, base, ref string) (string, error) {
	ref = strings.ReplaceAll(ref, "\\", "/")
	rel := path.Join(base, ref)
	if ref == "" || strings.HasPrefix(ref, "/") || strings.Contains(ref, ":") ||
		!filepath.IsLocal(filepath.FromSlash(rel)) || rel == "." {
		return "", fmt.Errorf("invalid project path %q", ref)
	}
	if _, err := root.Stat(filepath.FromSlash(rel)); err == nil {
		return rel, nil
	}
	current := "."
	for _, part := range strings.Split(rel, "/") {
		entries, err := fs.ReadDir(root.FS(), current)
		if err != nil {
			return "", fmt.Errorf("resolve %q: %w", rel, err)
		}
		match, ambiguous := "", false
		for _, e := range entries {
			if e.Name() == part {
				match = part
				ambiguous = false
				break
			}
			if strings.EqualFold(e.Name(), part) {
				if match != "" {
					ambiguous = true
				}
				match = e.Name()
			}
		}
		if ambiguous {
			return "", fmt.Errorf("ambiguous project path %q", rel)
		}
		if match == "" {
			return "", fmt.Errorf("missing project file %q", rel)
		}
		current = path.Join(current, match)
	}
	if _, err := root.Stat(filepath.FromSlash(current)); err != nil {
		return "", fmt.Errorf("resolve %q: %w", rel, err)
	}
	return current, nil
}
