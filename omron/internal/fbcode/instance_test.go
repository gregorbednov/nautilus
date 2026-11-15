package fbcode

import "testing"

func TestInstancePrivateAddressAndTransferBounds(t *testing.T) {
	for _, bad := range []struct {
		bank    int
		address string
	}{{0, "T3071"}, {0, "T4096"}, {0, "T03072"}, {1, "C4096"}, {3, "H511"}, {3, "H1408"}, {4, "H1407"}, {4, "H1536"}, {2, "#FFFF"}, {5, "H512"}} {
		if words, err := InstanceBaseMove(bad.bank, bad.address); words != nil || err == nil {
			t.Fatal("invalid private base admitted", bad)
		}
	}
	for _, n := range []int{-1, 2048} {
		if a, b, err := InstanceEnvelope(n); a != nil || b != nil || err == nil {
			t.Fatal("instance identity aliased", n)
		}
		if words, err := InstanceBodyCall(n); words != nil || err == nil {
			t.Fatal("body identity aliased", n)
		}
		if words, err := InstanceControl(n); words != nil || err == nil {
			t.Fatal("FI identity aliased", n)
		}
		for _, bit := range []bool{false, true} {
			for _, push := range []bool{false, true} {
				if words, err := InstanceTransfer(bit, push, n); words != nil || err == nil {
					t.Fatal("relative transfer aliased", n)
				}
			}
		}
	}
	if words, err := InstanceTransfer(false, true, 0); words != nil || err == nil {
		t.Fatal("unqualified word output admitted")
	}
}
