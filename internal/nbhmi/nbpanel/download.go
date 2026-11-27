package nbpanel

import (
	"bytes"
	"context"
	"fmt"
	"time"
)

const (
	nbSessionStart = 0x01
	nbSessionEnd   = 0x0f
	nbBeginFile    = 0x10
	nbFileBlock    = 0x11
	nbFinishFile   = 0x12
	nbFileSize     = 0x1a
	nbHMIDigest    = 0x22
	nbSetVersion   = 0x27
	nbShortTimeout = time.Second
	nbFlashTimeout = 90 * time.Second
)

type nbResource struct {
	name string
	data []byte
}

type nbDownloadPlan struct {
	runtime        [3]nbResource
	versions       [3]uint16
	driver         nbResource
	driverVersion  uint32
	printer        nbResource
	printerVersion uint32
	fontLibrary    nbResource
	fonts          []nbResource
	project        []nbResource
	integrity      []byte
}

func (pkg *Package) downloadPlan() (nbDownloadPlan, error) {
	var plan nbDownloadPlan
	if pkg == nil || pkg.ModelID != modelIDNB10W || len(pkg.data) == 0 {
		return plan, fmt.Errorf("expected a validated %s package", ModelName)
	}
	data := pkg.data
	if _, err := pkg.suffixOffsets(data); err != nil {
		return plan, fmt.Errorf("plan Ethernet download: %w", err)
	}
	at := pkg.project
	resource := func(name string, offset, size int) nbResource {
		return nbResource{name, data[offset : offset+size]}
	}
	for i, name := range [...]string{"gui", "comserver", "servo.bak"} {
		entry := 30 + 16*i
		version := u32(data, entry+4)
		if version == 0 || version > 65535 {
			return plan, fmt.Errorf("unsupported %s version %d", name, version)
		}
		plan.versions[i] = uint16(version)
		plan.runtime[i] = resource(name, u32(data, entry+12), u32(data, entry+8))
	}
	drivers := u32(data, at+112)
	driverAt, driverSize := u32(data, drivers+12), u32(data, drivers+8)
	if driverSize < 4 || !bytes.Equal(data[driverAt:driverAt+4], []byte("\x7fELF")) {
		return plan, fmt.Errorf("package is missing the NB10W Ethernet PLC driver")
	}
	plan.driver = resource("drv_p3.so", driverAt, driverSize)
	plan.driverVersion = uint32(u32(data, drivers+4))
	plan.printer = resource("lpt_drv.so", u32(data, at+132), u32(data, at+128))
	plan.printerVersion = uint32(u32(data, at+124))
	plan.fontLibrary = resource("libfreetype.so", u32(data, at+156), u32(data, at+152))
	fonts := u32(data, at+148)
	for i := range u32(data, at+144) {
		entry := fonts + 12*i
		plan.fonts = append(plan.fonts, resource(fmt.Sprintf("font_%d.ttf", i+1), u32(data, entry+8), u32(data, entry+4)))
	}
	plan.project = []nbResource{
		resource("uncompile", u32(data, at+140), u32(data, at+136)),
		resource("HMI0.hmi", pkg.hmi, pkg.hmiEnd-pkg.hmi),
	}
	// The package and Ethernet transfer use the same encoded NB.dat digest.
	plan.integrity = data[at+integrityField : at+integrityField+integritySize]
	return plan, nil
}

// Download sends a user project and its runtime resources to an NB10W over
// Ethernet. onResource is called after the panel acknowledges each file.
func (pkg *Package) Download(ctx context.Context, endpoint string, onResource func(name string, size int)) error {
	plan, err := pkg.downloadPlan()
	if err != nil {
		return err
	}
	client, err := dialNBDownload(ctx, endpoint)
	if err != nil {
		return err
	}
	defer client.close()
	version, err := client.version()
	if err != nil {
		return err
	}
	if version.ModelID != pkg.ModelID || version.Mode == 1 {
		return fmt.Errorf("panel model or mode is incompatible with download (model %d, mode %d)", version.ModelID, version.Mode)
	}
	if version.ServoVersion < 0x48c || plan.versions[2] < 0x48c {
		return fmt.Errorf("unsupported panel or package runtime version for Ethernet download")
	}

	transfer := func(file nbResource) error {
		if err := client.sendFile(file); err != nil {
			return fmt.Errorf("%s: %w", file.name, err)
		}
		if onResource != nil {
			onResource(file.name, len(file.data))
		}
		return nil
	}
	mode, err := client.beginDownload()
	if err != nil {
		return err
	}
	if err := client.downloadResources(plan, version, mode, transfer); err != nil {
		return fmt.Errorf("download stopped after opening the panel session: %w", err)
	}
	return nil
}

func (client *nbDownloadConn) beginDownload() (byte, error) {
	body, err := client.exchange(encodeNBFrame(nbSessionStart, []byte("\x01eView")), -1, 1, 4*nbShortTimeout)
	if err != nil {
		return 0, fmt.Errorf("open download session: %w", err)
	}
	if len(body) == 0 || body[0] != 1 && body[0] != 2 {
		return 0, fmt.Errorf("panel declined download session: %w", nbStatus(body, 0, 1))
	}
	mode := body[0]
	// The stock Ethernet client sends twelve bytes for this seven-byte frame.
	frame := append(encodeNBFrame(0x41, []byte{0}), 'V', 'i', 'e', 'w', 0x0d)
	if _, err := client.exchange(frame, -1, 1, 4*nbShortTimeout); err != nil {
		return 0, fmt.Errorf("initialize download: %w", err)
	}
	return mode, nil
}

func (client *nbDownloadConn) downloadResources(plan nbDownloadPlan, v PanelVersion, mode byte, transfer func(nbResource) error) error {
	deviceVersions := [3]uint16{v.GUIVersion, v.ComVersion, v.ServoVersion}
	for i, file := range plan.runtime {
		if plan.versions[i] == deviceVersions[i] {
			continue
		}
		if err := transfer(file); err != nil {
			return err
		}
		body := []byte{byte(i + 3), 0, 0, 0, 0}
		big.PutUint16(body[3:], plan.versions[i])
		if err := client.control("record "+file.name+" version", nbSetVersion, body); err != nil {
			return err
		}
	}
	for _, item := range []struct {
		file    nbResource
		query   byte
		version uint32
	}{
		{plan.driver, 0x05, plan.driverVersion},
		{plan.printer, 0x08, plan.printerVersion},
	} {
		body, err := client.exchange(encodeNBFrame(item.query, nil), -1, 3, nbShortTimeout)
		if err == nil {
			err = nbStatus(body, 0, 1)
		}
		if err != nil || len(body) < 5 {
			if err == nil {
				err = fmt.Errorf("incomplete resource version reply")
			}
			return fmt.Errorf("query %s: %w", item.file.name, err)
		}
		installed := uint32(big.Uint16(body[1:])) | uint32(big.Uint16(body[3:]))<<16
		if installed != item.version {
			if err := transfer(item.file); err != nil {
				return err
			}
		}
	}

	session := []byte{mode, 'e', 'V', 'i', 'e', 'w'}
	if err := client.control("prepare project without macros", 0x2a, session); err != nil {
		return err
	}
	size, err := client.exchange(encodeNBFrame(nbFileSize, []byte(plan.fontLibrary.name)), -1, 3, nbShortTimeout)
	if err != nil {
		return fmt.Errorf("query font library size: %w", err)
	}
	if len(size) < 4 {
		return fmt.Errorf("incomplete font library size reply")
	}
	if big.Uint32(size) != uint32(len(plan.fontLibrary.data)) {
		if err := transfer(plan.fontLibrary); err != nil {
			return err
		}
	}
	if err := client.control("prepare fonts", 0x2f, session); err != nil {
		return err
	}
	for _, group := range [][]nbResource{plan.fonts, plan.project} {
		for _, file := range group {
			if err := transfer(file); err != nil {
				return err
			}
		}
	}
	if err := client.control("verify compiled HMI", nbHMIDigest, plan.integrity); err != nil {
		return err
	}
	if err := client.control("finish download", nbSessionEnd, nil); err != nil {
		return err
	}
	if v.ServoVersion != plan.versions[2] {
		return client.control("finish runtime update", 0x07, session)
	}
	return nil
}

func (client *nbDownloadConn) control(stage string, command byte, payload []byte) error {
	body, err := client.exchange(encodeNBFrame(command, payload), -1, 1, nbFlashTimeout)
	if err == nil {
		err = nbStatus(body, 0, 1)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", stage, err)
	}
	return nil
}

func (client *nbDownloadConn) sendFile(file nbResource) error {
	blocks := (len(file.data) + nbChunkSize - 1) / nbChunkSize
	begin := make([]byte, 8, 8+len(file.name))
	big.PutUint32(begin[2:], uint32(len(file.data)))
	big.PutUint16(begin[6:], uint16(blocks+2))
	begin = append(begin, file.name...)
	reply, err := client.exchange(encodeNBFrame(nbBeginFile, begin), -1, 3, nbShortTimeout)
	if err == nil {
		err = nbStatus(reply, 0, 1)
	}
	if err != nil {
		return fmt.Errorf("start transfer: %w", err)
	}
	block := make([]byte, 2, nbChunkSize+2)
	for i := range blocks {
		seq := i + 1
		big.PutUint16(block, uint16(seq))
		chunk := file.data[i*nbChunkSize : min(seq*nbChunkSize, len(file.data))]
		payload := append(block, chunk...)
		reply, err = client.exchange(encodeNBFrame(nbFileBlock, payload), seq, 4, nbShortTimeout)
		if err == nil {
			err = nbStatus(reply, 2, 1)
		}
		if err != nil {
			return fmt.Errorf("block %d of %d: %w", seq, blocks, err)
		}
	}
	seq := blocks + 1
	var finish [6]byte
	big.PutUint16(finish[:2], uint16(seq))
	big.PutUint32(finish[2:], nbSum(file.data))
	if err := client.finishFile(seq, finish[:]); err != nil {
		return fmt.Errorf("flash write could not be confirmed: %w", err)
	}
	return nil
}

func (client *nbDownloadConn) finishFile(seq int, body []byte) error {
	// Once the panel may be writing flash, wait for its terminal reply.
	// A retry could start a second write while the first one is still in flight.
	if err := client.send(encodeNBFrame(nbFinishFile, body), time.Now().Add(nbShortTimeout)); err != nil {
		return err
	}
	deadline := time.Now().Add(nbFlashTimeout)
	writing := false
	for {
		command, reply, err := client.receive(nbFinishFile, seq, deadline, true)
		if err != nil {
			return err
		}
		if command == nbWriteProgress {
			writing = true
			continue
		}
		if len(reply) >= 3 && reply[2] == 2 {
			if writing {
				return nil
			}
			writing = true
			continue
		}
		return nbStatus(reply, 2, 1)
	}
}
