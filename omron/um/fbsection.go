package um

import (
	"encoding/binary"
	"fmt"
	"slices"

	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

// FBSectionInstruction describes one measured CPU21 generated instruction.
type FBSectionInstruction struct {
	OffsetWords int      `json:"offsetWords"`
	Text        string   `json:"text"`
	Words       []uint32 `json:"words"`
}

type FBCodeSection struct {
	Kind             string                 `json:"kind"`
	Number           uint16                 `json:"number"`
	ImageOffsetWords uint32                 `json:"imageOffsetWords"`
	Instructions     []FBSectionInstruction `json:"instructions"`
}

type sectionReader struct {
	remaining []uint32
	offset    int
	rows      []FBSectionInstruction
}

func (r *sectionReader) take(text string, code ...uint32) bool {
	if len(code) == 0 || len(r.remaining) < len(code) || !slices.Equal(r.remaining[:len(code)], code) {
		return false
	}
	r.rows = append(r.rows, FBSectionInstruction{r.offset, text, code})
	r.remaining = r.remaining[len(code):]
	r.offset += len(code)
	return true
}

// doubleSource returns a decoded source operand and its canonical encoding.
// Generated math uses BR3/IR63 displacements or low-word-first literals.
func doubleSource(w []uint32) (string, []uint32) {
	if len(w) == 0 {
		return "", nil
	}
	switch w[0] & 0xfffff000 {
	case 0x693c3000, 0x6937f000:
		base, reg := uint32(0x693c3000), "BR3"
		if w[0]&0xfffff000 == 0x6937f000 {
			base, reg = 0x6937f000, "IR63"
		}
		disp := int32(w[0]<<20) >> 20
		return fmt.Sprintf("%d,%s", disp, reg), []uint32{base | uint32(disp)&0xfff}
	}
	if len(w) >= 2 && w[0]>>16 == 0x6940 && w[1]>>16 == 0x6140 {
		value := w[1]<<16 | uint32(uint16(w[0]))
		return fmt.Sprintf("#%X", value), []uint32{0x69400000 | uint32(uint16(value)), 0x61400000 | value>>16}
	}
	return "", nil
}

var arithmeticNames = map[uint32]string{
	0xb3410300: "#+F(454)", 0xb3410302: "#-F(455)", 0xb3410304: "#*F(456)", 0xb3410306: "#/F(457)",
}

func (r *sectionReader) floatInstruction() bool {
	w := r.remaining
	if len(w) < 3 {
		return false
	}
	switch w[0] {
	case 0xb341030a:
		return r.take("#FLT(452) 1,BR3 -2,IR63", 0xb341030a, 0x613c3001, 0x5d37fffe)
	case 0xb3410141:
		dest := w[2] & 0xfff
		if dest > 2047 {
			return false
		}
		return r.take(fmt.Sprintf("#MOVL(498) -2,IR63 %d,BR3", dest), 0xb3410141, 0x6937fffe, 0x5d3c3000|dest)
	}
	name, ok := arithmeticNames[w[0]]
	if !ok {
		return false
	}
	a, first := doubleSource(w[1:])
	b, second := doubleSource(w[1+len(first):])
	if len(first) == 0 || len(second) == 0 {
		return false
	}
	code := append([]uint32{w[0]}, first...)
	code = append(code, second...)
	return r.take(name+" "+a+" "+b+" -2,IR63", append(code, 0x5d37fffe)...)
}

func (r *sectionReader) definition(number int) bool {
	entry, exit, err := fbcode.DefinitionEnvelope(8, number)
	if err != nil || !r.take(fmt.Sprintf("FBBN %d", number), entry...) ||
		!r.take("~+L(401) +3 IR63 IR63", 0x9b7f0201, 0x69460003, 0x61460000, 0x6907f000, 0x5d07f000) {
		return false
	}
	for r.floatInstruction() {
	}
	return r.take("FBBEND", 0xb0000814) &&
		r.take("~+L(401) -3 IR63 IR63", 0x9b7f0201, 0x6946fffd, 0x6146ffff, 0x6907f000, 0x5d07f000) &&
		r.take("FBBRET", exit...)
}

func (r *sectionReader) instance(number int) bool {
	entry, exit, err := fbcode.InstanceEnvelope(number)
	if err != nil || !r.take(fmt.Sprintf("FBIN %d", number), entry...) {
		return false
	}
	for bank, addr := range []string{"T3072", "C3072", "#9FFF", "", "H1408"} {
		op := "~MOVRW(561)"
		if bank == 2 {
			op = "~MOVL(498)"
		}
		if bank >= 3 {
			op = "~MOVR(560)"
		}
		if bank == 3 {
			if len(r.remaining) < 3 {
				return false
			}
			addr = fmt.Sprintf("H%d", int64(r.remaining[1])-0x5000d800)
		}
		code, err := fbcode.InstanceBaseMove(bank, addr)
		if err != nil || !r.take(fmt.Sprintf("%s %s BR%d", op, addr, bank), code...) {
			return false
		}
	}
	control, _ := fbcode.InstanceControl(number)
	for i, text := range []string{fmt.Sprintf("LD FI%d", number), fmt.Sprintf("RSET FI%d", number), "#SET 1,BR3"} {
		if !r.take(text, control[i]...) {
			return false
		}
	}
	popWord, _ := fbcode.InstanceTransfer(false, false, 1)
	popBit, _ := fbcode.InstanceTransfer(true, false, 0)
	pushBit, _ := fbcode.InstanceTransfer(true, true, 1)
	if !r.take("~MOV(021) ,-IR63 1,BR3", popWord...) || !r.take("~BMOV(559) ,-IR62 ,BR3", popBit...) || len(r.remaining) < 3 {
		return false
	}
	body := int(r.remaining[1] & 0x7ff)
	call, _ := fbcode.InstanceBodyCall(body)
	return r.take(fmt.Sprintf("FBBCALL &%d", body), call...) && r.take("~BMOV(559) 1,BR3 ,IR62+", pushBit...) &&
		r.take("~MOVL(498) 2,BR3 ,IR63++", 0xbb7f0141, 0x693c3002, 0x5d17f00a) && r.take("FBIRET", exit...)
}

// DecodeCPU21FBSections verifies every instruction against the original code.
// It accepts the measured floating-point definitions and scalar instance frames.
func (im *N1Image) DecodeCPU21FBSections() ([]FBCodeSection, error) {
	blocks, err := im.FunctionBlocks()
	if err != nil {
		return nil, err
	}
	if binary.BigEndian.Uint32(im.raw[6*4:])&0x40000000 != 0 {
		return nil, fmt.Errorf("%w: CPU21 FB decoding requires BCD mode", ErrN1FunctionBlocks)
	}
	sections := []FBCodeSection{}
	add := func(kind string, number uint16, start uint32, r *sectionReader, ok bool) error {
		if !ok || len(r.remaining) != 0 {
			return fmt.Errorf("%w: %s %d at image word %d", ErrN1FunctionBlocks, kind, number, start+uint32(r.offset))
		}
		sections = append(sections, FBCodeSection{kind, number, start, r.rows})
		return nil
	}
	for _, d := range blocks.Definitions {
		r := sectionReader{remaining: d.Code}
		if err := add("definition", d.Number, d.OffsetWords+uint32(len(d.Header)), &r, r.definition(int(d.BodyNumber))); err != nil {
			return nil, err
		}
	}
	for _, i := range blocks.Instances {
		r := sectionReader{remaining: i.Code}
		if err := add("instance", i.Number, i.OffsetWords+uint32(len(i.Header)), &r, r.instance(int(i.Number))); err != nil {
			return nil, err
		}
	}
	return sections, nil
}
