package matrix

import "testing"

func TestKindString(t *testing.T) {
	tests := []struct {
		k    Kind
		want string
	}{
		{KindFR, "FR"}, {KindNFR, "NFR"}, {KindTE, "TE"},
		{KindTC, "TC"}, {KindNEW, "NEW"}, {KindR, "R"},
	}
	for _, tt := range tests {
		if string(tt.k) != tt.want {
			t.Errorf("Kind = %q, want %q", tt.k, tt.want)
		}
	}
}

func TestSeverityValues(t *testing.T) {
	if SeverityCritical == SeverityImportant {
		t.Fatal("Critical and Important must differ")
	}
	if SeverityAdvisory == SeverityImportant {
		t.Fatal("Advisory and Important must differ")
	}
}
