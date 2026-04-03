package goquery

import "testing"

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"items", "Items"},
		{"order_items", "OrderItems"},
		{"category", "Category"},
		{"created_at", "CreatedAt"},
		{"a", "A"},
		{"already_Pascal", "AlreadyPascal"},
		{"with-dash", "WithDash"},
		{"__leading", "Leading"},
		{"multiple__underscores", "MultipleUnderscores"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToPascalCase(tt.input)
			if got != tt.want {
				t.Errorf("ToPascalCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
