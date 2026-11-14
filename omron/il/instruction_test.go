package il

import (
	"errors"
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		source, mnemonic, modifier, function string
		operands                             []string
	}{
		{"LD W0.01", "LD", "", "", []string{"W0.01"}},
		{" @MOV(021) #FFFF D10 ", "MOV", "@", "021", []string{"#FFFF", "D10"}},
		{"%LD CIO0.00", "LD", "%", "", []string{"CIO0.00"}},
		{"+(400) D0 &1 D2", "+", "", "400", []string{"D0", "&1", "D2"}},
		{"END(001)", "END", "", "001", []string{}},
		{"MOVL(498)\t*D0\t@D1", "MOVL", "", "498", []string{"*D0", "@D1"}},
		{`MOV "a b" "c"" d"`, "MOV", "", "", []string{`"a b"`, `"c"" d"`}},
		{"LD Подъёмник.Вход[2]", "LD", "", "", []string{"Подъёмник.Вход[2]"}},
		{"FUTURE(9999) E2_100 IR0,++", "FUTURE", "", "9999", []string{"E2_100", "IR0,++"}},
	} {
		got, err := Parse(tc.source)
		if err != nil {
			t.Fatalf("%q: %v", tc.source, err)
		}
		if got.Source != tc.source || got.Mnemonic != tc.mnemonic || got.Modifier != tc.modifier || got.Function != tc.function || !reflect.DeepEqual(got.Operands, tc.operands) {
			t.Fatalf("%q: %+v", tc.source, got)
		}
	}
}
func TestSyntaxErrors(t *testing.T) {
	for _, source := range []string{"", "\t", "@", "MOV()", "MOV(ABC)", "MOV(021", "MOV(021))", "MOV) D0", "LD\nOUT", "LD\x00D0", "LD \"open", string([]byte{0xff})} {
		_, err := Parse(source)
		var syntax *SyntaxError
		if !errors.As(err, &syntax) || syntax.Column < 1 {
			t.Fatalf("%q: %v", source, err)
		}
	}
	_, err := Parse(`  MOV(0x1) D0`)
	var syntax *SyntaxError
	if !errors.As(err, &syntax) || syntax.Column != 8 {
		t.Fatalf("wrong location: %v", err)
	}
}
func FuzzParse(f *testing.F) {
	for _, s := range []string{"LD W0.00", "@MOV(021) #0 D0", `MOV "a b" D0`, "END(001)"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, source string) {
		got, err := Parse(source)
		if err == nil && (got.Source != source || got.Mnemonic == "" || got.Operands == nil) {
			t.Fatal("lost instruction data")
		}
	})
}
