package tasks

import (
	"testing"
)

func TestSanitizeFilenamePreservesExtension(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"report.pdf", "report.pdf"},
		{"Design and Access.pdf", "Design and Access.pdf"},
		{"file (1).pdf", "file 1.pdf"},
		{"hello_world-v2.pdf", "hello_world-v2.pdf"},
		{"../../../etc/passwd", "......etcpasswd"},
		{"file\x00name.pdf", "filename.pdf"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewCookieJar(t *testing.T) {
	jar := newCookieJar()
	if jar == nil {
		t.Error("newCookieJar() returned nil")
	}
}
