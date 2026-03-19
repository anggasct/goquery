package goquery

import (
	"fmt"
	"strings"
)

type Dialect int

const (
	Postgres Dialect = iota
	MySQL
)

// DetectDialect maps a database driver name to a Dialect.
// Recognized names: "postgres", "pgx", "postgresql", "pg" → Postgres; "mysql" → MySQL.
// Defaults to Postgres for unrecognized drivers.
func DetectDialect(driverName string) Dialect {
	switch strings.ToLower(strings.TrimSpace(driverName)) {
	case "mysql":
		return MySQL
	default:
		return Postgres
	}
}

// DialectPtr returns a pointer to d, useful for setting Options.Dialect explicitly.
func DialectPtr(d Dialect) *Dialect {
	return &d
}

type SQLClause struct {
	SQL  string
	Args []any
}

type ClauseBuilder struct {
	Dialect    Dialect
	FieldToCol map[string]string
}

func (b *ClauseBuilder) ResolveColumn(field string) string {
	field = strings.TrimSpace(field)
	if field == "" {
		return ""
	}
	if b.FieldToCol == nil {
		return field
	}
	if col, ok := b.FieldToCol[field]; ok {
		col = strings.TrimSpace(col)
		if col != "" {
			return col
		}
	}
	return field
}

func (b *ClauseBuilder) SearchClause(spec Spec, argOffset int) SQLClause {
	keyword := strings.TrimSpace(spec.Q)
	if keyword == "" || len(spec.SearchFields) == 0 {
		return SQLClause{}
	}

	likeOp := "ILIKE"
	if b.Dialect == MySQL {
		likeOp = "LIKE"
	}

	clauses := make([]string, 0, len(spec.SearchFields))
	args := make([]any, 0, len(spec.SearchFields))
	for _, field := range spec.SearchFields {
		col := b.ResolveColumn(field)
		if col == "" {
			continue
		}
		clauses = append(clauses, fmt.Sprintf("%s %s %s", col, likeOp, b.placeholder(argOffset+len(args))))
		args = append(args, "%"+keyword+"%")
	}
	if len(clauses) == 0 {
		return SQLClause{}
	}
	return SQLClause{
		SQL:  "(" + strings.Join(clauses, " OR ") + ")",
		Args: args,
	}
}

func (b *ClauseBuilder) FilterClauses(spec Spec, argOffset int) []SQLClause {
	out := make([]SQLClause, 0, len(spec.Filters))
	offset := argOffset

	for _, f := range spec.Filters {
		col := b.ResolveColumn(f.Field)
		if col == "" || len(f.Values) == 0 {
			continue
		}

		var clause SQLClause
		switch f.Operator {
		case "eq":
			clause = SQLClause{
				SQL:  fmt.Sprintf("%s = %s", col, b.placeholder(offset)),
				Args: []any{f.Values[0]},
			}
			offset++
		case "in":
			placeholders := make([]string, len(f.Values))
			for i := range f.Values {
				placeholders[i] = b.placeholder(offset + i)
			}
			clause = SQLClause{
				SQL:  fmt.Sprintf("%s IN (%s)", col, strings.Join(placeholders, ", ")),
				Args: f.Values,
			}
			offset += len(f.Values)
		case "like":
			likeOp := "ILIKE"
			if b.Dialect == MySQL {
				likeOp = "LIKE"
			}
			clause = SQLClause{
				SQL:  fmt.Sprintf("%s %s %s", col, likeOp, b.placeholder(offset)),
				Args: []any{"%" + fmt.Sprint(f.Values[0]) + "%"},
			}
			offset++
		case "gt":
			clause = SQLClause{
				SQL:  fmt.Sprintf("%s > %s", col, b.placeholder(offset)),
				Args: []any{f.Values[0]},
			}
			offset++
		case "gte":
			clause = SQLClause{
				SQL:  fmt.Sprintf("%s >= %s", col, b.placeholder(offset)),
				Args: []any{f.Values[0]},
			}
			offset++
		case "lt":
			clause = SQLClause{
				SQL:  fmt.Sprintf("%s < %s", col, b.placeholder(offset)),
				Args: []any{f.Values[0]},
			}
			offset++
		case "lte":
			clause = SQLClause{
				SQL:  fmt.Sprintf("%s <= %s", col, b.placeholder(offset)),
				Args: []any{f.Values[0]},
			}
			offset++
		case "between":
			if len(f.Values) == 2 {
				clause = SQLClause{
					SQL:  fmt.Sprintf("%s BETWEEN %s AND %s", col, b.placeholder(offset), b.placeholder(offset+1)),
					Args: []any{f.Values[0], f.Values[1]},
				}
				offset += 2
			}
		default:
			continue
		}

		if clause.SQL != "" {
			out = append(out, clause)
		}
	}
	return out
}

func (b *ClauseBuilder) SortSQL(spec Spec, defaultSort string) string {
	if len(spec.Sort) == 0 {
		return strings.TrimSpace(defaultSort)
	}

	parts := make([]string, 0, len(spec.Sort))
	for _, s := range spec.Sort {
		col := b.ResolveColumn(s.Field)
		if col == "" {
			continue
		}
		dir := "ASC"
		if s.Desc {
			dir = "DESC"
		}
		clause := col + " " + dir
		if b.Dialect == Postgres {
			clause += " NULLS LAST"
		}
		parts = append(parts, clause)
	}
	return strings.Join(parts, ", ")
}

func (b *ClauseBuilder) PageLimitOffset(spec Spec) (limit, offset int) {
	if spec.Limit <= 0 || spec.Page <= 0 {
		return spec.Limit, 0
	}
	return spec.Limit, (spec.Page - 1) * spec.Limit
}

func (b *ClauseBuilder) WhereClauses(spec Spec, argOffset int) SQLClause {
	var parts []string
	var args []any

	search := b.SearchClause(spec, argOffset)
	if search.SQL != "" {
		parts = append(parts, search.SQL)
		args = append(args, search.Args...)
	}

	filters := b.FilterClauses(spec, argOffset+len(args))
	for _, f := range filters {
		parts = append(parts, f.SQL)
		args = append(args, f.Args...)
	}

	if len(parts) == 0 {
		return SQLClause{}
	}
	return SQLClause{
		SQL:  strings.Join(parts, " AND "),
		Args: args,
	}
}

func (b *ClauseBuilder) placeholder(index int) string {
	if b.Dialect == MySQL {
		return "?"
	}
	return fmt.Sprintf("$%d", index+1)
}
