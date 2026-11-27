package nbpanel

import "testing"

func TestConfirmationRequiresBothStockResponses(t *testing.T) {
	const recordSize = 64
	image := make([]byte, recordSize*2)
	screens := make([]Screen, confirmScreenID+1)
	for i := range 2 {
		start := i * recordSize
		row := image[start:]
		little.PutUint32(row[8:], 0x02000000)
		little.PutUint32(row[12:], 0xff02)
		little.PutUint32(row[16:], confirmResponseWord)
		little.PutUint32(row[44:], 1)
		little.PutUint32(row[48:], uint32(i+1))
		screens[confirmScreenID].Components = append(screens[confirmScreenID].Components,
			Component{Type: 0x302, Offset: start, Size: recordSize})
	}
	pkg := Package{Screens: screens}
	flags, wait, err := pkg.confirmation(image, 90)
	if err != nil || flags != 0x1002 || wait != 90 {
		t.Fatalf("got flags %#x and wait %d, err %v", flags, wait, err)
	}
	little.PutUint32(image[recordSize+16:], confirmResponseWord+1)
	if _, _, err := pkg.confirmation(image, 90); err == nil {
		t.Fatal("dialog with an unrelated Cancel address was accepted")
	}
	if flags, wait, err := pkg.confirmation(image, 0); err != nil || flags != 2 || wait != 30 {
		t.Fatalf("disabled confirmation changed control fields: %#x, %d, %v", flags, wait, err)
	}
}
