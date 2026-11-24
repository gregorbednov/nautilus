package compiler

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/um"
)

// ImageBuildVerification describes a local reproduction, not historical origin,
// author authentication, source-setting correctness or hardware acceptance.
// SettingsSHA256 hashes encoding/json output of N1ImageSettings, not a JSON file.
type ImageBuildVerification struct {
	Method           string `json:"method"`
	Model            string `json:"model"`
	UnitVersion      string `json:"unitVersion"`
	InOutBOOLVersion string `json:"inOutBOOLVersion,omitempty"`
	TimerCounterMode string `json:"timerCounterMode"`
	Resource         string `json:"resource"`
	SourceEncoding   string `json:"sourceEncoding"`
	SourceSHA256     string `json:"sourceSHA256"`
	SettingsSHA256   string `json:"canonicalSettingsSHA256"`
	ImageSHA256      string `json:"imageSHA256"`
	ImageBytes       int    `json:"imageBytes"`
}

// VerifiedImageBuild can only be initialized by successful local compilation
// and exact comparison. JSON reports are deliberately not accepted as proofs.
// Zero/nil values never match, and no mutable image or recipe is exposed.
type VerifiedImageBuild struct {
	report  ImageBuildVerification
	image   []byte
	archive []byte
}

func (v *VerifiedImageBuild) Report() ImageBuildVerification {
	if v == nil {
		return ImageBuildVerification{}
	}
	return v.report
}

func (v *VerifiedImageBuild) Matches(model string, image []byte) bool {
	return v != nil && len(v.image) != 0 && v.report.Model == model && bytes.Equal(v.image, image)
}

// SourceArchive returns a copy of the FB source built from the same source as
// the executable image. A program with no FB needs no archive.
func (v *VerifiedImageBuild) SourceArchive() []byte {
	if v == nil {
		return nil
	}
	return bytes.Clone(v.archive)
}

// BuildCXPImageForDownload compiles both parts before the caller opens a PLC
// connection. FB layouts the archive writer cannot represent fail here.
func BuildCXPImageForDownload(profile codec.Profile, source []byte, encoding, resource string, settings N1ImageSettings) (*VerifiedImageBuild, []byte, error) {
	doc, image, err := compileProofImage(profile, source, encoding, resource, settings)
	if err != nil {
		return nil, nil, err
	}
	proof, err := newImageBuildProof("local-download-build-v1", profile, source, encoding, resource, settings, image.data)
	if err != nil {
		return nil, nil, err
	}
	if len(image.ResourceBody.FBPrefix.Definitions.Words) != 0 {
		if resource != "Resource[0]" {
			return nil, nil, errors.New("FB archive download requires Resource[0]")
		}
		switch profile.Model {
		case codec.CJ1MCPU21:
			proof.archive, err = doc.BuildFBL("")
		case codec.CJ1HCPU65:
			proof.archive, err = doc.BuildProgramsOnlyFBL("")
		default:
			return nil, nil, errors.New("unsupported FB archive download model")
		}
		if err != nil {
			return nil, nil, fmt.Errorf("build SRCPRGRM.FBL: %w", err)
		}
	}
	return proof, image.Bytes(), nil
}

// VerifyCXPImageBuild re-parses the exact input bytes and recompiles with the
// supplied recipe. Equal minimal images may reproduce on both CPU profiles;
// this establishes compatibility with a recipe, never a unique original model.
func VerifyCXPImageBuild(profile codec.Profile, source []byte, encoding, resource string, settings N1ImageSettings, candidate []byte) (*VerifiedImageBuild, error) {
	if len(candidate) > um.MaxSize {
		return nil, errors.New("build verification candidate exceeds size limit")
	}
	_, image, err := compileProofImage(profile, source, encoding, resource, settings)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(image.data, candidate) {
		return nil, errors.New("candidate differs from local recipe recompilation")
	}
	return newImageBuildProof("local-recompile-exact-bytes-v1", profile, source, encoding, resource, settings, candidate)
}

func compileProofImage(profile codec.Profile, source []byte, encoding, resource string, settings N1ImageSettings) (*cxp.Document, *N1Image, error) {
	if profile.TimerCounterMode != codec.TimerModeBCD && profile.TimerCounterMode != codec.TimerModeBinary {
		return nil, nil, errors.New("image build verification requires explicit BCD or binary mode")
	}
	doc, err := cxp.ReadEncoding(source, encoding)
	if err != nil {
		return nil, nil, err
	}
	image, err := CompileCXPImage(profile, doc, resource, settings)
	return doc, image, err
}

func newImageBuildProof(method string, profile codec.Profile, source []byte, encoding, resource string, settings N1ImageSettings, candidate []byte) (*VerifiedImageBuild, error) {
	canonical, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}
	digest := func(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
	report := ImageBuildVerification{Method: method, Model: profile.Model, UnitVersion: profile.UnitVersion, InOutBOOLVersion: settings.InOutBOOLVersion, TimerCounterMode: string(profile.TimerCounterMode), Resource: resource, SourceEncoding: encoding, SourceSHA256: digest(source), SettingsSHA256: digest(canonical), ImageSHA256: digest(candidate), ImageBytes: len(candidate)}
	return &VerifiedImageBuild{report: report, image: bytes.Clone(candidate)}, nil
}
