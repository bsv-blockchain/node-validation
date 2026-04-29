package txgen

import "testing"

func TestEstimateSize_singleInputSingleOutput(t *testing.T) {
	out := Output{Script: make([]byte, 25), Satoshis: 1}
	got := EstimateSize(1, []Output{out})
	want := sizeTxOverhead + sizeP2PKHInput + sizePerOutputOverhead + 25
	if got != want {
		t.Errorf("size: got %d want %d", got, want)
	}
}

func TestEstimateSize_twoInputsThreeOutputs(t *testing.T) {
	outs := []Output{
		{Script: make([]byte, 25)},
		{Script: make([]byte, 25)},
		{Script: make([]byte, 25)},
	}
	got := EstimateSize(2, outs)
	want := sizeTxOverhead + 2*sizeP2PKHInput + 3*(sizePerOutputOverhead+25)
	if got != want {
		t.Errorf("size: got %d want %d", got, want)
	}
}

func TestEstimateSize_largeScriptCrossesVarint(t *testing.T) {
	out := Output{Script: make([]byte, 1000), Satoshis: 0}
	got := EstimateSize(1, []Output{out})
	// 10 + 148 + 9 + 1000 + 2 (varint extra) = 1169
	if got != 1169 {
		t.Errorf("size: got %d want 1169", got)
	}
}

func TestComputeFee_roundsUp(t *testing.T) {
	cases := []struct {
		size  int
		rate  uint64
		wantF uint64
	}{
		{1000, 500, 500}, // exactly 500
		{1, 500, 1},      // ceil(0.5)
		{0, 500, 0},      // free
		{500, 1000, 500}, // 500 * 1000 / 1000
	}
	for _, c := range cases {
		if got := ComputeFee(c.size, c.rate); got != c.wantF {
			t.Errorf("ComputeFee(%d, %d) = %d want %d", c.size, c.rate, got, c.wantF)
		}
	}
}
