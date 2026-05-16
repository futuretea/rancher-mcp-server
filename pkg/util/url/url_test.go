package url

import "testing"

func TestNormalizeRancherURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"strips v3 suffix", "https://rancher.example.com/v3", "https://rancher.example.com"},
		{"no v3 suffix", "https://rancher.example.com", "https://rancher.example.com"},
		{"v3 in middle", "https://rancher.example.com/v3/subpath", "https://rancher.example.com/v3/subpath"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeRancherURL(tt.url)
			if got != tt.want {
				t.Errorf("NormalizeRancherURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestGetNormanURL(t *testing.T) {
	got := GetNormanURL("https://rancher.example.com/v3")
	want := "https://rancher.example.com/v3"
	if got != want {
		t.Errorf("GetNormanURL() = %q, want %q", got, want)
	}
}

func TestGetSteveURL(t *testing.T) {
	got := GetSteveURL("https://rancher.example.com/v3", "c-abc123")
	want := "https://rancher.example.com/k8s/clusters/c-abc123"
	if got != want {
		t.Errorf("GetSteveURL() = %q, want %q", got, want)
	}
}
