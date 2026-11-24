// Command cpu21sample rebuilds the archived Temperature/Humidity application
// from recovered instructions, allocation and metadata. The recipe has no UM
// input. It produces the compiler-side candidate with zero service words.
package main

import (
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/il"
	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

var profile = codec.Profile{Model: codec.CJ1MCPU21, TimerCounterMode: codec.TimerModeBCD}

// All arguments are fixed parts of the recovered recipe. An encoder rejection
// is a recipe error; no output file exists until the full build succeeds.
func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

type definition struct {
	name   string
	number int
}

var temperature = definition{"Temperature", 0}
var humidity = definition{"Humidity", 1}

func (d definition) body() int { return 1024 + d.number }

type instance struct {
	number, holding int
	def             definition
}

var instances = []instance{{1, 536, temperature}, {2, 512, temperature}, {3, 520, temperature}, {4, 528, temperature}, {5, 580, humidity}, {6, 544, temperature}}

//go:embed Temperature.st
var temperatureSource string

//go:embed Humidity.st
var humiditySource string

func (d definition) record() []uint32 {
	source := humiditySource
	if d == temperature {
		source = temperatureSource
	}
	return must(compiler.CompileSTFloatDefinition(profile, source, d.number, d.body())).RecordWords
}
func (i instance) record() []uint32 {
	code, exit, err := fbcode.InstanceEnvelope(i.number)
	if err != nil {
		panic(err)
	}
	for bank, addr := range []string{"T3072", "C3072", "#9FFF", fmt.Sprintf("H%d", i.holding), "H1408"} {
		code = append(code, must(fbcode.InstanceBaseMove(bank, addr))...)
	}
	for _, op := range must(fbcode.InstanceControl(i.number)) {
		code = append(code, op...)
	}
	code = append(code, must(fbcode.InstanceTransfer(false, false, 1))...)
	code = append(code, must(fbcode.InstanceTransfer(true, false, 0))...)
	code = append(code, must(fbcode.InstanceBodyCall(i.def.body()))...)
	code = append(code, must(fbcode.InstanceTransfer(true, true, 1))...)
	code = append(code, 0xbb7f0141, 0x693c3002, 0x5d17f00a) // MOVL return through IR63
	code = append(code, exit...)
	header := make([]uint32, 10)
	header[0], header[2], header[6] = uint32(i.number), uint32(len(header)+len(code)), 8
	return append(header, code...)
}

func taskBlock() []uint32 {
	var code []uint32
	emit := func(line string) {
		code = append(code, must(codec.EncodeInstruction(profile, must(il.Parse(line))))...)
	}
	for _, site := range []struct {
		inst   instance
		input  uint32
		output uint32
		coil   string
	}{
		{instances[5], 2097, 100, "W0.02"}, {instances[4], 2098, 200, "W0.03"},
	} {
		emit("LD CF113")
		for _, op := range must(fbcode.WordResultCall(site.input, site.output, site.inst.number, site.inst.def.body())) {
			code = append(code, op.Words...)
		}
		emit("OUT " + site.coil)
	}
	for _, line := range []string{"LD CF113", "SET 2090.00", "MOV D300 2091", "CMP #DEAD #DEAD", "END"} {
		emit(line)
	}
	const number = 256
	block := make([]uint32, 288)
	block[number] = uint32(len(block))
	header := make([]uint32, 32)
	header[0], header[26], header[27] = number, uint32(37+len(code)), 32
	// НоваяЗадача1 in the original single-byte task-name field.
	for i, b := range []byte("\xcd\xee\xe2\xe0\xff\xc7\xe0\xe4\xe0\xf7\xe0\x31") {
		header[1+i/3] |= uint32(b) << uint(16-8*(i%3))
	}
	block = append(block, header...)
	block = append(block, 0, 0xbbc00f43, 0x61440000|number, 0x65440000)
	block = append(block, code...)
	return append(block, 0xbb800f48)
}

func build() []byte {
	defs := []uint32{2, 0, 0, 0}
	for _, d := range []definition{humidity, temperature} {
		defs = append(defs, d.record()...)
	}
	inst := make([]uint32, 8)
	inst[0] = uint32(len(instances))
	for _, i := range instances {
		inst = append(inst, i.record()...)
	}
	task := taskBlock()
	memory := codec.N1FBMemorySettings{Bases: []codec.N1MemoryBase{
		{Address: "T3072", Size: 1024}, {Address: "C3072", Size: 1024}, {Address: "DIFF8191"}, {Address: "H512", Size: 896}, {Address: "H1408", Size: 128},
	}, Stack: codec.N1MemoryStack{Address: "AC0", FirstSize: 64, TotalSize: 1536}}
	header := must(codec.EncodeN1Header(profile, codec.N1TaskOptions{}, codec.N1HeaderLayout{DefinitionWords: len(defs), InstanceWords: len(inst), TaskBlockWords: len(task)}, &memory))
	cfg := make([]uint32, 288)
	for i := range cfg {
		class := uint32(0x2000)
		if i >= 256 {
			class = 0x1000
		}
		cfg[i] = class | uint32(i)
	}
	cfg[256] |= 0xc000 // present + operation start
	var data []byte
	for _, section := range [][]uint32{header, cfg, defs, inst, task} {
		for _, w := range section {
			data = binary.BigEndian.AppendUint32(data, w)
		}
	}
	return data
}

func run() error {
	var data []byte
	switch {
	case len(os.Args) == 2:
		data = build()
	case len(os.Args) == 4 && os.Args[1] == "--readback":
		var err error
		data, err = buildReadback(os.Args[2])
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("usage: cpu21sample [--readback PROJECT.cxp] NEW.n1")
	}
	f, err := os.OpenFile(os.Args[len(os.Args)-1], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	fmt.Printf("%d bytes sha256 %x\n", len(data), sha256.Sum256(data))
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
