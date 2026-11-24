package compiler

import (
	"encoding/binary"
	"fmt"
	"regexp"
	"slices"
	"strconv"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/il"
	"github.com/joyautomation/nautilus/omron/internal/fbcode"
	"github.com/joyautomation/nautilus/omron/um"
	"golang.org/x/text/encoding/charmap"
)

type STILDefinition struct {
	Name       string `json:"name"`
	BodyNumber int    `json:"bodyNumber"`
}

// STILProgram is the editable source for a single CPU21 cyclic task using
// scalar ST FBs. Array order determines the on-disk order of FB records.
type STILProgram struct {
	Definitions []STILDefinition `json:"definitions"`
	Instances   []STILInstance   `json:"instances"`
	Task        struct {
		Name           string   `json:"name"`
		Number         int      `json:"number"`
		OperationStart *bool    `json:"operationStart"`
		Instructions   []string `json:"instructions"`
	} `json:"task"`
}

type STILInstance struct {
	Number     int    `json:"number"`
	Definition string `json:"definition"`
	Holding    int    `json:"holding"`
}

func floatInstanceRecord(i STILInstance, body int) ([]uint32, error) {
	code, end, err := fbcode.InstanceEnvelope(i.Number)
	if err != nil {
		return nil, err
	}
	for bank, address := range []string{"T3072", "C3072", "#9FFF", fmt.Sprintf("H%d", i.Holding), "H1408"} {
		move, err := fbcode.InstanceBaseMove(bank, address)
		if err != nil {
			return nil, err
		}
		code = append(code, move...)
	}
	control, err := fbcode.InstanceControl(i.Number)
	if err != nil {
		return nil, err
	}
	for _, op := range control {
		code = append(code, op...)
	}
	input, _ := fbcode.InstanceTransfer(false, false, 1)
	enable, _ := fbcode.InstanceTransfer(true, false, 0)
	call, err := fbcode.InstanceBodyCall(body)
	if err != nil {
		return nil, err
	}
	eno, _ := fbcode.InstanceTransfer(true, true, 1)
	for _, op := range [][]uint32{input, enable, call, eno, {0xbb7f0141, 0x693c3002, 0x5d17f00a}, end} {
		code = append(code, op...)
	}
	header := make([]uint32, 10)
	header[0], header[2], header[6] = uint32(i.Number), uint32(len(header)+len(code)), 8
	return append(header, code...), nil
}

func compileSTILTask(profile codec.Profile, lines []string, bodies map[int]int) ([]uint32, error) {
	var words []uint32
	previous := ""
	for i, line := range lines {
		ins, err := il.Parse(line)
		if err != nil {
			return nil, fmt.Errorf("instruction %d: %w", i+1, err)
		}
		if previous == "END" || (previous == "" && ins.Mnemonic != "LD") || (previous == "CALL" && ins.Mnemonic != "OUT") {
			return nil, fmt.Errorf("unexpected %s at instruction %d", ins.Mnemonic, i+1)
		}
		if ins.Mnemonic == "CALL" {
			if previous != "LD" || len(ins.Operands) != 3 || ins.Function != "" || ins.Modifier != "" {
				return nil, fmt.Errorf("instruction %d: expected CALL instance CIO-word D-word after LD", i+1)
			}
			num, err := strconv.Atoi(ins.Operands[0])
			body, exists := bodies[num]
			if err != nil || !exists {
				return nil, fmt.Errorf("instruction %d: unknown instance %s", i+1, ins.Operands[0])
			}
			in, e1 := codec.ParseWordAddress(ins.Operands[1])
			out, e2 := codec.ParseWordAddress(ins.Operands[2])
			if e1 != nil || e2 != nil || in.Area != codec.CIO || out.Area != codec.Data {
				return nil, fmt.Errorf("instruction %d: expected CIO input and DM result", i+1)
			}
			frame, err := fbcode.WordResultCall(uint32(in.Word), uint32(out.Word), num, body)
			if err != nil {
				return nil, err
			}
			for _, op := range frame {
				words = append(words, op.Words...)
			}
		} else {
			switch ins.Mnemonic {
			case "LD", "OUT", "SET", "MOV", "CMP", "END":
			default:
				return nil, fmt.Errorf("instruction %d: unsupported %s", i+1, ins.Mnemonic)
			}
			code, err := codec.EncodeInstruction(profile, ins)
			if err != nil {
				return nil, fmt.Errorf("instruction %d: %w", i+1, err)
			}
			words = append(words, code...)
		}
		if len(words) > MaxInstructions-37 {
			return nil, ErrSourceLimit
		}
		previous = ins.Mnemonic
	}
	if previous != "END" {
		return nil, fmt.Errorf("task requires a terminal END")
	}
	return words, nil
}

var stTaskName = regexp.MustCompile(`^[\pL_][\pL\pN_]*$`)

func CompileSTILImage(profile codec.Profile, program STILProgram, sources map[string]string, settings N1ImageSettings) (*N1Image, error) {
	if profile != (codec.Profile{Model: codec.CJ1MCPU21, TimerCounterMode: codec.TimerModeBCD}) {
		return nil, codec.ErrUnqualifiedProfile
	}
	bases := []codec.N1MemoryBase{{Address: "T3072", Size: 1024}, {Address: "C3072", Size: 1024}, {Address: "DIFF8191"}, {Address: "H512", Size: 896}, {Address: "H1408", Size: 128}}
	if !slices.Equal(settings.Memory.Bases, bases) || settings.Memory.Stack != (codec.N1MemoryStack{Address: "AC0", FirstSize: 64, TotalSize: 1536}) || settings.Options != (codec.N1TaskOptions{}) || settings.NextEnable != [7]uint32{} || settings.InOutBOOLVersion != "" {
		return nil, fmt.Errorf("unsupported ST/IL image settings")
	}
	t := program.Task
	name, err := charmap.Windows1251.NewEncoder().String(t.Name)
	if err != nil || !stTaskName.MatchString(t.Name) || len(name) > 24 || t.OperationStart == nil || t.Number < 256 || t.Number > 287 {
		return nil, fmt.Errorf("invalid cyclic task name, number or startup setting")
	}
	if len(program.Definitions) == 0 || len(program.Definitions) > 1024 || len(program.Definitions) != len(settings.DefinitionNumbers) || len(sources) != len(program.Definitions) {
		return nil, fmt.Errorf("definition names and explicit IDs must correspond")
	}
	defs := []uint32{uint32(len(program.Definitions)), 0, 0, 0}
	ids, bodyIDs := map[int]bool{}, map[int]bool{}
	byName := map[string]int{}
	for _, d := range program.Definitions {
		number, ok := settings.DefinitionNumbers[d.Name]
		if !ok || ids[number] || bodyIDs[d.BodyNumber] || d.BodyNumber < 1024 {
			return nil, fmt.Errorf("invalid or duplicate definition binding %s", d.Name)
		}
		rec, err := CompileSTFloatDefinition(profile, sources[d.Name], number, d.BodyNumber)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", d.Name, err)
		}
		if rec.Name != d.Name {
			return nil, fmt.Errorf("ST declaration %s differs from %s", rec.Name, d.Name)
		}
		ids[number], bodyIDs[d.BodyNumber], byName[d.Name] = true, true, d.BodyNumber
		if len(defs) > MaxInstructions-len(rec.RecordWords) {
			return nil, ErrSourceLimit
		}
		defs = append(defs, rec.RecordWords...)
	}
	if len(program.Instances) == 0 || len(program.Instances) > 896/8 {
		return nil, fmt.Errorf("instance count outside holding bank")
	}
	occupied := map[int]bool{}
	bodies := map[int]int{}
	instances := make([]uint32, 8)
	instances[0] = uint32(len(program.Instances))
	for _, i := range program.Instances {
		body, ok := byName[i.Definition]
		_, duplicate := bodies[i.Number]
		if !ok || duplicate || i.Number < 1 || i.Holding < 512 || i.Holding > 1408-8 {
			return nil, fmt.Errorf("invalid instance binding or placement: %d", i.Number)
		}
		for w := i.Holding; w < i.Holding+8; w++ {
			if occupied[w] {
				return nil, fmt.Errorf("instance %d overlaps H%d", i.Number, w)
			}
			occupied[w] = true
		}
		record, err := floatInstanceRecord(i, body)
		if err != nil {
			return nil, err
		}
		bodies[i.Number] = body
		instances = append(instances, record...)
	}
	code, err := compileSTILTask(profile, t.Instructions, bodies)
	if err != nil {
		return nil, err
	}
	task := &cxp.TaskSource{Name: name, Number: t.Number}
	if *t.OperationStart {
		task.OperationStart = 1
	}
	config := makeTaskConfiguration("", []*cxp.TaskSource{task})
	block := make([]uint32, 288)
	block[t.Number] = uint32(len(block))
	block = append(block, wrapTaskRecord(t.Number, name, code)...)
	header, err := codec.EncodeN1Header(profile, settings.Options, codec.N1HeaderLayout{DefinitionWords: len(defs), InstanceWords: len(instances), TaskBlockWords: len(block)}, &settings.Memory)
	if err != nil {
		return nil, err
	}
	var data []byte
	for _, section := range [][]uint32{header, config.Words, defs, instances, block} {
		for _, w := range section {
			data = binary.BigEndian.AppendUint32(data, w)
		}
	}
	image, err := um.ParseN1(data)
	if err != nil {
		return nil, err
	}
	if _, err = image.FunctionBlocks(); err != nil {
		return nil, err
	}
	span := TaskRecordSpan{Program: t.Name, Number: t.Number, OffsetWords: len(data)/4 - len(block) + 288, LengthWords: len(block) - 288}
	return &N1Image{HeaderWords: header, Tasks: []TaskRecordSpan{span}, data: data, report: image.Summary()}, nil
}
