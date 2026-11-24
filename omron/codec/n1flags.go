package codec

// N1TaskOptions controls the qualified header flags for ordinary unprotected
// task images. It does not describe FB-derived flags, task protection checks,
// automatic allocation, passwords, or memory initialization. Those states must
// be qualified separately before building a complete project image.
type N1TaskOptions struct {
	IndependentIRDR     bool `json:"independentIRDR"`
	ExtendedTIMHold     bool `json:"extendedTIMHold"`
	CopyProhibited      bool `json:"copyProhibited"`
	OverwriteProhibited bool `json:"overwriteProhibited"`
}

// EncodeN1TaskFlags encodes the qualified options at N1 header word 6. Timer
// mode must be explicit. These flags alone do not establish image completeness,
// CPU compatibility, or permission to upload an image to hardware.
func EncodeN1TaskFlags(profile Profile, options N1TaskOptions) (uint32, error) {
	if err := profile.validate(); err != nil {
		return 0, err
	}
	if profile.TimerCounterMode != TimerModeBCD && profile.TimerCounterMode != TimerModeBinary {
		return 0, ErrUnqualifiedProfile
	}
	var flags uint32
	if options.IndependentIRDR {
		flags |= 0x20000000
	}
	if profile.TimerCounterMode == TimerModeBinary {
		flags |= 0x40000000
	}
	if options.ExtendedTIMHold {
		flags |= 0x08000000
	}
	if options.CopyProhibited {
		flags |= 0x2000
	}
	if options.OverwriteProhibited {
		flags |= 0x4000
	}
	return flags, nil
}
