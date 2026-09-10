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

func TestRedactCredentials(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"userinfo credentials", "https://user:s3cret@rancher.example.com/v3", "https://***@rancher.example.com/v3"},
		{"username only", "https://token@rancher.example.com/v3", "https://***@rancher.example.com/v3"},
		{"unescaped at sign in password", "https://user:p@ss@rancher.example.com/v3", "https://***@rancher.example.com/v3"},
		{"unparseable url", "https://user:pa/ss@rancher.example.com/v3", "https://***@rancher.example.com/v3"},
		{"no credentials", "https://rancher.example.com/v3", "https://rancher.example.com/v3"},
		{"at sign in path", "https://rancher.example.com/path@name", "https://rancher.example.com/path@name"},
		{"no scheme", "rancher.example.com", "rancher.example.com"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RedactCredentials(tt.url); got != tt.want {
				t.Errorf("RedactCredentials(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
