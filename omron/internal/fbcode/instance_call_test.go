package fbcode

import "testing"

func TestInstanceCallRejectsUnsupportedDomain(t *testing.T) {
	for _, test := range []struct {
		selector             uint32
		instance, definition int
	}{
		{4, 0, 1024}, {8, -1, 1024}, {8, 0, -1}, {8, 2048, 1024}, {8, 0, 2048},
		{8, 65535, 1024}, {8, 0, 65535}, {8, int(^uint(0) >> 1), 1024},
	} {
		if words, err := InstanceCall(test.selector, test.instance, test.definition); err == nil || words != nil {
			t.Fatal("accepted unqualified instance call", test)
		}
	}
}
