package tasks

import (
	"testing"
)

func TestCleanJSONResponseMarkdownFencesVariant(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "fences with language tag",
			input: "```json\n{\"score\": 5}\n```",
			want:  `{"score": 5}`,
		},
		{
			name:  "fences without language tag",
			input: "```\n{\"score\": 5}\n```",
			want:  `{"score": 5}`,
		},
		{
			name:  "think tags with content before JSON",
			input: "<think>\nLet me analyze this carefully.\n</think>\n\n{\"score\": 7}",
			want:  `{"score": 7}`,
		},
		{
			name:  "text surrounding JSON",
			input: "Here is my analysis:\n{\"tags\": [\"residential\"]}\nEnd of response.",
			want:  `{"tags": ["residential"]}`,
		},
		{
			name:  "no JSON at all",
			input: "I cannot analyze this.",
			want:  "I cannot analyze this.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanJSONResponse(tt.input)
			if got != tt.want {
				t.Errorf("cleanJSONResponse() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClampEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		val  int
		min  int
		max  int
		want int
	}{
		{"negative val with positive range", -10, 1, 10, 1},
		{"large positive val", 999, 1, 10, 10},
		{"val equals min", 1, 1, 10, 1},
		{"val equals max", 10, 1, 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clamp(tt.val, tt.min, tt.max)
			if got != tt.want {
				t.Errorf("clamp(%d, %d, %d) = %d, want %d", tt.val, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestLimitSlicePreservesOrder(t *testing.T) {
	input := []string{"a", "b", "c", "d", "e"}
	result := limitSlice(input, 3)

	expected := []string{"a", "b", "c"}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("index %d: got %q, want %q", i, v, expected[i])
		}
	}
}

func TestFormatMetadataOrdering(t *testing.T) {
	// Single entry to avoid map ordering issues
	metadata := map[string]string{
		"Applicant": "John Smith",
	}

	result := formatMetadata(metadata)
	if result != "- Applicant: John Smith" {
		t.Errorf("unexpected result: %q", result)
	}
}
