package goquery

import (
	"strings"
	"unicode"
)

// ToPascalCase converts a snake_case or camelCase string to PascalCase.
//
//	"items"       → "Items"
//	"order_items" → "OrderItems"
//	"createdAt"   → "CreatedAt"
func ToPascalCase(s string) string {
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))

	upper := true
	for _, r := range s {
		if r == '_' || r == '-' {
			upper = true
			continue
		}
		if upper {
			b.WriteRune(unicode.ToUpper(r))
			upper = false
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
