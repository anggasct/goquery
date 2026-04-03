package sqlxx

import (
	"fmt"
	"strings"

	"github.com/anggasct/goquery"
)

type whereCond struct {
	sql  string
	args []any
}

// WhereBuilder accumulates WHERE conditions using ? placeholders.
// Placeholders are automatically converted to the correct dialect
// (e.g., $1, $2 for Postgres) when resolved.
type WhereBuilder struct {
	conds []whereCond
}

// Add appends a WHERE condition. Use ? for placeholders regardless of dialect.
//
//	w.Add("deleted_at IS NULL")
//	w.Add("status = ?", "published")
//	w.Add("age BETWEEN ? AND ?", 18, 65)
func (w *WhereBuilder) Add(condition string, args ...any) *WhereBuilder {
	w.conds = append(w.conds, whereCond{sql: condition, args: args})
	return w
}

// IsEmpty returns true if no conditions have been added.
func (w *WhereBuilder) IsEmpty() bool {
	return len(w.conds) == 0
}

// resolve converts all ? placeholders to dialect-appropriate form and returns
// the combined SQL parts and flattened args.
func (w *WhereBuilder) resolve(dialect goquery.Dialect, argOffset int) (parts []string, args []any) {
	if len(w.conds) == 0 {
		return nil, nil
	}

	parts = make([]string, len(w.conds))
	n := argOffset

	for i, c := range w.conds {
		if dialect == goquery.MySQL {
			parts[i] = c.sql
		} else {
			var sb strings.Builder
			for j := 0; j < len(c.sql); j++ {
				if c.sql[j] == '?' {
					n++
					sb.WriteString(fmt.Sprintf("$%d", n))
				} else {
					sb.WriteByte(c.sql[j])
				}
			}
			parts[i] = sb.String()
		}
		args = append(args, c.args...)
	}

	return parts, args
}
