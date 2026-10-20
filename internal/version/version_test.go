package version

import "testing"

func TestNewer(t *testing.T) {
	defer func(n string) { Number = n }(Number)

	Number = ""
	if Release() || Newer("9.9.9") {
		t.Error("una build di sviluppo non deve aggiornarsi")
	}

	Number = "1.2.3"
	tests := []struct {
		other string
		want  bool
	}{
		{"1.2.4", true},
		{"v1.3.0", true},
		{"2.0.0", true},
		{"1.2.3", false},
		{"1.2.2", false},
		{"1.3.0-rc.1", true},
		{"1.2.3-rc.1", false}, // una pre-release viene prima della versione finale
		{"", false},
		{"latest", false},
	}
	for _, tt := range tests {
		if got := Newer(tt.other); got != tt.want {
			t.Errorf("Newer(%q) = %v, voglio %v", tt.other, got, tt.want)
		}
	}
}
