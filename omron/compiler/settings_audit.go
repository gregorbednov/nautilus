package compiler

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
)

type SourceSetting struct {
	Path  string `json:"path"`
	Line  int    `json:"line"`
	Value string `json:"value"`
}
type SettingsComparison struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Source string `json:"source,omitempty"`
	Recipe string `json:"recipe,omitempty"`
	Detail string `json:"detail"`
}

// DeclarationValue inventories column index 5 of the observed eight-column
// declaration format. Its runtime initialization semantics remain unqualified.
type DeclarationValue struct {
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Variable string `json:"variable"`
	Column5  string `json:"column5"`
}

// NativeLibraryProfileState is a retained local library observation after SetPLC.
// It is not the current audit context, a device response, or a target version.
type NativeLibraryProfileState struct {
	DefaultVersionBytes      [3]uint8 `json:"defaultVersionBytes"`
	CurrentVersionBytes      [3]uint8 `json:"currentVersionBytes"`
	ExtendedProtect          uint32   `json:"extendedProtect"`
	Origin                   string   `json:"origin"`
	SourceEventLogSHA256     string   `json:"sourceEventLogSHA256"`
	HardwareUnitVersionKnown bool     `json:"hardwareUnitVersionKnown"`
}

// NativeProfileCandidate records an archived SetPLC worker context, not the
// current project context or a connected device's capabilities/unit version.
type NativeProfileCandidate struct {
	LibraryState         *NativeLibraryProfileState `json:"libraryState"`
	Origin               string                     `json:"origin"`
	Selector             uint32                     `json:"selector"`
	FamilyMaskLow        uint32                     `json:"familyMaskLow"`
	FamilyMaskHigh       uint32                     `json:"familyMaskHigh"`
	SourceEventLogSHA256 string                     `json:"sourceEventLogSHA256"`
	UnitVersionKnown     bool                       `json:"unitVersionKnown"`
}

type ImageSettingsAudit struct {
	NativeProfileCandidate *NativeProfileCandidate `json:"nativeProfileCandidate,omitempty"`
	SavedCS2SupportVersion string                  `json:"savedCS2SupportVersion,omitempty"`

	BR2Selection               string              `json:"br2Selection"`
	BR2DefaultCandidate        *codec.N1MemoryBase `json:"br2DefaultCandidate,omitempty"`
	SourceUseFunctionBlockFlag string              `json:"sourceUseFunctionBlockFlag,omitempty"`
	FBSupportProfileVerified   bool                `json:"fbSupportProfileVerified"`

	StackSelection       string               `json:"stackSelection"`
	StackSelectionOrigin string               `json:"stackSelectionOrigin"`
	StackCallCandidate   *codec.N1MemoryStack `json:"stackCallCandidate,omitempty"`

	Format                 string               `json:"format"`
	SourceSHA256           string               `json:"sourceSHA256"`
	Resource               string               `json:"resource"`
	Model                  string               `json:"model"`
	SourceEncoding         string               `json:"sourceEncoding"`
	SavedMemory            []SourceSetting      `json:"savedMemory"`
	DeclarationValues      []DeclarationValue   `json:"declarationValues"`
	DeclarationRows        int                  `json:"declarationRows"`
	Checks                 []SettingsComparison `json:"checks"`
	SourceSettingsVerified bool                 `json:"sourceSettingsVerified"`
	StorageInitialized     bool                 `json:"storageInitialized"`
	GlobalStorageAudited   bool                 `json:"globalStorageAudited"`
	NetworkAccess          bool                 `json:"networkAccess"`
	HardwareQualified      bool                 `json:"hardwareQualified"`
	WriteQualified         bool                 `json:"writeQualified"`
}

var auditResourcePath = regexp.MustCompile(`^Resource\[[0-9]+\]$`)

// AuditCXPImageSettings compares saved textual FB-memory fields with an explicit
// recipe. It applies only the pinned missing-UseDefault reader rule; no other
// missing defaults or runtime size units are inferred. It does not parse
// opaque PLC settings, generate initial data, compile code or access a PLC.
func AuditCXPImageSettings(profile codec.Profile, source []byte, encoding, resource string, settings N1ImageSettings) (*ImageSettingsAudit, error) {
	if !auditResourcePath.MatchString(resource) {
		return nil, errors.New("invalid settings-audit resource path")
	}
	if _, err := codec.EncodeN1FBMemory(profile, settings.Memory); err != nil {
		return nil, err
	}
	doc, err := cxp.ReadEncoding(source, encoding)
	if err != nil {
		return nil, err
	}
	// Reject duplicate paths within the selected resource, including duplicate
	// ancestors; never let the first/last occurrence silently determine a value.
	fields := map[string]cxp.Field{}
	duplicate := false
	doc.Walk(func(path string, f cxp.Field) {
		if path == resource || strings.HasPrefix(path, resource+".") {
			if _, ok := fields[path]; ok {
				duplicate = true
			}
			fields[path] = f
		}
	})
	if duplicate {
		return nil, errors.New("duplicate fields in selected settings-audit resource")
	}
	if f, ok := fields[resource]; !ok || f.Kind != "object" {
		return nil, errors.New("settings-audit resource not found")
	}
	report := &ImageSettingsAudit{Format: "cj1-source-settings-audit", SourceSHA256: fmt.Sprintf("%x", sha256.Sum256(source)), Resource: resource, Model: profile.Model, SourceEncoding: doc.Encoding, SavedMemory: []SourceSetting{}, DeclarationValues: []DeclarationValue{}, Checks: []SettingsComparison{}}
	// These masks come from archived profile events, not source CS2SupportVersion
	// or fresh reads. Do not derive CurrentVersion or hardware unit version.
	mask := uint32(0x400)
	if profile.Model == codec.CJ1HCPU65 {
		mask = 0x80000
	}
	report.NativeProfileCandidate = &NativeProfileCandidate{Origin: "retained-disposable-native-library-worker", Selector: 8, FamilyMaskLow: mask, SourceEventLogSHA256: "b2f1813d2a2a55d0270cb9ec88c715353f8afecd7c88f6490f19cf48b618c9a8"}

	// Retained observation of the pinned library in an isolated local worker.
	// Do not turn its default/current bytes or flag into target capability proof.
	report.NativeProfileCandidate.LibraryState = &NativeLibraryProfileState{
		DefaultVersionBytes: [3]uint8{4, 0, 0}, CurrentVersionBytes: [3]uint8{4, 0, 0},
		ExtendedProtect: 0, Origin: "retained-local-profile-state-worker",
		SourceEventLogSHA256: "b3a4139b313dfdc0a240c7d288bb80a4da8c6824f030d6499e7e25c069a65a98",
	}

	if saved, ok := fields[resource+".CS2SupportVersion"]; ok && saved.Kind == "scalar" {
		report.SavedCS2SupportVersion = saved.Value
	}
	report.Checks = append(report.Checks, SettingsComparison{ID: "source-version-provenance", State: "unverified", Source: report.SavedCS2SupportVersion, Detail: "CS2SupportVersion is a saved compatibility stamp; CurrentVersion/default/connected unit version must not be inferred from this field"})
	memory := resource + ".FBMemory"
	doc.Walk(func(path string, f cxp.Field) {
		if strings.HasPrefix(path, memory+".") && f.Kind == "scalar" {
			report.SavedMemory = append(report.SavedMemory, SourceSetting{path, f.Line, f.Value})
		}
	})
	compare := func(id, path, want string) {
		check := SettingsComparison{ID: id, State: "unverified", Recipe: want, Detail: "missing saved field; no default inferred"}
		if f, ok := fields[path]; ok {
			if f.Kind != "scalar" {
				check.Detail = "saved field has unsupported shape"
			} else {
				check.Source = f.Text()
				check.State = "blocked"
				check.Detail = "saved text differs from explicit recipe; no conversion or override applied"
				if check.Source == want {
					check.State = "passed"
					check.Detail = "exact textual agreement only; runtime mapping remains unqualified"
				}
			}
		}
		report.Checks = append(report.Checks, check)
	}
	compare("register-count", memory+".NumRegisters", "5")
	for i, base := range settings.Memory.Bases {
		if i != 2 {
			compare(fmt.Sprintf("bank-%d", i), fmt.Sprintf("%s.BR[%d]", memory, i), base.Address+","+strconv.Itoa(base.Size))
			continue
		}
		// Both the saved-memory reader/writer and the compiler bridge skip
		// index 2. Even a present source field does not override this bank.
		report.BR2Selection = "initialized-default-candidate"
		report.BR2DefaultCandidate = &codec.N1MemoryBase{Address: "DIFF8191", Size: 0}
		sourceText := ""
		if field, ok := fields[memory+".BR[2]"]; ok && field.Kind == "scalar" {
			sourceText = field.Text()
		}
		report.Checks = append(report.Checks, SettingsComparison{ID: "bank-2", State: "unverified", Source: sourceText, Recipe: base.Address + "," + strconv.Itoa(base.Size), Detail: "pinned source reader/writer and bridge skip BR2; saved text is not applied; active initialized profile remains unqualified"})
		state := "blocked"
		if base == *report.BR2DefaultCandidate {
			state = "passed"
		}
		report.Checks = append(report.Checks, SettingsComparison{ID: "bank-2-default-table", State: state, Source: "DIFF8191,0", Recipe: base.Address + "," + strconv.Itoa(base.Size), Detail: "agreement with pinned default bank table only; this is not proof of an active CPU profile or branch"})
	}
	if flag, ok := fields[resource+".UseFunctionBlockFlag"]; ok && flag.Kind == "scalar" {
		report.SourceUseFunctionBlockFlag = flag.Value
	}
	report.Checks = append(report.Checks, SettingsComparison{ID: "fb-support-profile", State: "unverified", Detail: "register initialization tests IsFunctionBlockSupported(true), including DefaultVersion on the version-dependent branch; setting UseFunctionBlockFlag tests false/CurrentVersion; saved flag alone proves neither"})
	// Pinned CX-P reader defaults missing UseDefault to true. The bridge
	// initializes KBase registers, then calls SetStackRegister only if false.
	// Candidate values describe this static branch, not an active CPU profile.
	report.StackSelection = "unknown"
	if stack, ok := fields[memory+".StackSettings"]; ok && stack.Kind == "object" {
		useDefault := true
		report.StackSelectionOrigin = "native-missing-field-default"
		if flag, found := fields[memory+".StackSettings.UseDefault"]; found {
			if flag.Kind != "scalar" {
				return nil, errors.New("unsupported stack UseDefault shape")
			}
			switch strings.ToLower(flag.Value) {
			case "true", "1":
				useDefault = true
			case "false", "0":
				useDefault = false
			default:
				return nil, errors.New("unsupported stack UseDefault value")
			}
			report.StackSelectionOrigin = "explicit-source-field"
		}
		if useDefault {
			report.StackSelection = "use-default"
			report.StackCallCandidate = &codec.N1MemoryStack{Address: "AC0", FirstSize: 64, TotalSize: 1536}
			report.Checks = append(report.Checks, SettingsComparison{ID: "stack-address", State: "unverified", Source: fields[memory+".StackSettings.StartAddress"].Text(), Recipe: settings.Memory.Stack.Address, Detail: "saved address is inactive under UseDefault; it is not an applied D0/AC0 conflict"})
			state := "blocked"
			if *report.StackCallCandidate == settings.Memory.Stack {
				state = "passed"
			}
			report.Checks = append(report.Checks, SettingsComparison{ID: "stack-default-table", State: state, Source: "AC0/64/1536", Recipe: fmt.Sprintf("%s/%d/%d", settings.Memory.Stack.Address, settings.Memory.Stack.FirstSize, settings.Memory.Stack.TotalSize), Detail: "agreement with pinned default table only; profile/FB-mode branch activation still requires qualification"})
		} else {
			report.StackSelection = "explicit"
			compare("stack-address", memory+".StackSettings.StartAddress", settings.Memory.Stack.Address)
			number := func(name string) (int, bool) {
				f, ok := fields[memory+".StackSettings."+name]
				if !ok || f.Kind != "scalar" {
					return 0, false
				}
				n, e := strconv.Atoi(f.Value)
				return n, e == nil && n >= 0 && n <= 4096 && strconv.Itoa(n) == f.Value
			}
			word, wordOK := number("WordSize")
			bit, bitOK := number("BitSize")
			address, addressOK := fields[memory+".StackSettings.StartAddress"]
			if wordOK && bitOK && word+bit <= 4096 && addressOK && address.Kind == "scalar" {
				report.StackCallCandidate = &codec.N1MemoryStack{Address: address.Text(), FirstSize: bit, TotalSize: word + bit}
				state := "blocked"
				if bit == settings.Memory.Stack.FirstSize && word+bit == settings.Memory.Stack.TotalSize {
					state = "passed"
				}
				report.Checks = append(report.Checks, SettingsComparison{ID: "stack-call-arguments", State: state, Source: fmt.Sprintf("%d/%d", bit, word+bit), Recipe: fmt.Sprintf("%d/%d", settings.Memory.Stack.FirstSize, settings.Memory.Stack.TotalSize), Detail: "static bridge passes BitSize then WordSize+BitSize; this does not qualify runtime capacity or initialization"})
			} else {
				report.Checks = append(report.Checks, SettingsComparison{ID: "stack-call-arguments", State: "unverified", Detail: "explicit stack fields outside supported audit shape/range"})
			}
		}
	} else {
		compare("stack-address", memory+".StackSettings.StartAddress", settings.Memory.Stack.Address)
	}

	report.Checks = append(report.Checks,
		SettingsComparison{ID: "stack-size-units", State: "unverified", Detail: "explicit call formula is known; source/profile/native-address units and branch activation remain unqualified"},
		SettingsComparison{ID: "shared-sfc-memory", State: "unverified", Detail: "ShareSFCMemoryWithFBNonRetain and SFCMemory behavior are not qualified"},
		SettingsComparison{ID: "opaque-plc-settings", State: "unverified", Detail: "PLC.Settings.Data, timer/options and device configuration are not mapped by this audit"},
		SettingsComparison{ID: "declaration-initialization", State: "unverified", Detail: "declaration column5 values are inventoried but no initial storage data is generated"},
		SettingsComparison{ID: "global-storage-and-operands", State: "unverified", Detail: "global/local declarations, actual memory effects and instruction operands require separate audit"},
	)
	// Inventory observed declaration lists without requiring supported FB graphics.
	// Do not label nonempty literals as generated initialization or assume that a
	// zero/false literal is redundant with current or retained PLC memory.
	for path, f := range fields {
		if !strings.HasPrefix(path, resource+".FunctionBlocks.") || !strings.HasSuffix(path, ".FunctionBlock.POUHeader.PouVarList") {
			continue
		}
		if f.Kind != "list" || len(f.Raw) > cxp.MaxSymbolBytes {
			return nil, errors.New("unsupported declaration list in settings audit")
		}
		for i, row := range strings.Split(f.Raw, "\n") {
			row = strings.TrimSpace(row)
			if row == "" {
				continue
			}
			columns := strings.SplitN(row, ",", 8)
			if len(columns) != 8 || !strings.HasSuffix(columns[7], ";") {
				return nil, errors.New("unsupported declaration row in settings audit")
			}
			for _, column := range columns[:7] {
				if strings.ContainsAny(column, "\"\x11") {
					return nil, errors.New("quoted declaration metadata is not qualified for settings audit")
				}
			}
			report.DeclarationRows++
			if report.DeclarationRows > cxp.MaxSymbolRows {
				return nil, errors.New("settings-audit declaration limit")
			}
			value := strings.TrimSpace(columns[5])
			if value != "" {
				report.DeclarationValues = append(report.DeclarationValues, DeclarationValue{Path: path, Line: f.Line + 2 + i, Variable: strings.TrimSpace(columns[1]), Column5: value})
			}
		}
	}
	// Deterministic ordering is independent of Go map traversal.
	sort.Slice(report.DeclarationValues, func(i, j int) bool {
		a, b := report.DeclarationValues[i], report.DeclarationValues[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Line < b.Line
	})
	return report, nil
}
