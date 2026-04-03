package sqlxx

import (
	"context"
	"fmt"
	"strings"

	"github.com/anggasct/goquery"
	"github.com/jmoiron/sqlx"
)

// FilterFunc generates a custom WHERE condition for a filter field.
// Use this for filters that need subqueries or non-standard SQL.
type FilterFunc func(w *WhereBuilder, f goquery.Filter)

type Options struct {
	FieldToCol  map[string]string
	DefaultSort string
	Dialect     *goquery.Dialect          // nil = auto-detect from db connection
	Scope       func(w *WhereBuilder)     // dynamic WHERE conditions with ? placeholders
	Select      string                    // columns to select (default: "*")
	FilterScope map[string]FilterFunc     // custom SQL handlers for specific filter fields
}

type Clauses struct {
	Where   string
	OrderBy string
	Limit   int
	Offset  int
	Args    []any
}

func resolveDialect(db *sqlx.DB, opts Options) goquery.Dialect {
	if opts.Dialect != nil {
		return *opts.Dialect
	}
	return goquery.DetectDialect(db.DriverName())
}

func Build(spec goquery.Spec, opts Options) Clauses {
	var dialect goquery.Dialect
	if opts.Dialect != nil {
		dialect = *opts.Dialect
	}
	b := &goquery.ClauseBuilder{
		Dialect:    dialect,
		FieldToCol: opts.FieldToCol,
	}

	var whereParts []string
	var allArgs []any

	// Apply scope (dynamic WHERE conditions)
	if opts.Scope != nil {
		wb := &WhereBuilder{}
		opts.Scope(wb)
		if !wb.IsEmpty() {
			parts, args := wb.resolve(dialect, len(allArgs))
			whereParts = append(whereParts, parts...)
			allArgs = append(allArgs, args...)
		}
	}

	// Separate custom-scoped filters from default filters
	specForClauses := spec
	if len(opts.FilterScope) > 0 {
		wb := &WhereBuilder{}
		var defaultFilters []goquery.Filter
		for _, f := range spec.Filters {
			if fn, ok := opts.FilterScope[f.Field]; ok {
				fn(wb, f)
			} else {
				defaultFilters = append(defaultFilters, f)
			}
		}
		if !wb.IsEmpty() {
			parts, args := wb.resolve(dialect, len(allArgs))
			whereParts = append(whereParts, parts...)
			allArgs = append(allArgs, args...)
		}
		specForClauses.Filters = defaultFilters
	}

	// Goquery clauses (search + default filters) with arg offset
	where := b.WhereClauses(specForClauses, len(allArgs))
	if where.SQL != "" {
		whereParts = append(whereParts, where.SQL)
		allArgs = append(allArgs, where.Args...)
	}

	whereSQL := ""
	if len(whereParts) > 0 {
		whereSQL = strings.Join(whereParts, " AND ")
	}

	orderBy := b.SortSQL(spec, opts.DefaultSort)
	limit, offset := b.PageLimitOffset(spec)

	return Clauses{
		Where:   whereSQL,
		OrderBy: orderBy,
		Limit:   limit,
		Offset:  offset,
		Args:    allArgs,
	}
}

func Paginate[T any](
	ctx context.Context,
	db *sqlx.DB,
	spec goquery.Spec,
	from string,
	opts Options,
) (goquery.PageResult[T], error) {
	// Auto-detect dialect from db if not explicitly set
	if opts.Dialect == nil {
		d := resolveDialect(db, opts)
		opts.Dialect = &d
	}
	c := Build(spec, opts)

	sel := opts.Select
	if sel == "" {
		sel = "*"
	}
	baseQuery := "SELECT " + sel + " FROM " + from
	countQuery := "SELECT COUNT(*) FROM " + from

	if spec.Page <= 0 || spec.Limit == -1 {
		dataSQL := baseQuery
		if c.Where != "" {
			dataSQL += " WHERE " + c.Where
		}
		if c.OrderBy != "" {
			dataSQL += " ORDER BY " + c.OrderBy
		}
		if spec.Limit > 0 {
			dataSQL += fmt.Sprintf(" LIMIT %d", spec.Limit)
		}

		var items []T
		if err := sqlx.SelectContext(ctx, db, &items, dataSQL, c.Args...); err != nil {
			return goquery.PageResult[T]{}, err
		}
		n := int64(len(items))
		return goquery.PageResult[T]{
			Items: items,
			Meta:  goquery.BuildPageMeta(n, 1, len(items)),
		}, nil
	}

	countSQL := countQuery
	if c.Where != "" {
		countSQL += " WHERE " + c.Where
	}

	var total int64
	if err := sqlx.GetContext(ctx, db, &total, countSQL, c.Args...); err != nil {
		return goquery.PageResult[T]{}, err
	}

	dataSQL := baseQuery
	if c.Where != "" {
		dataSQL += " WHERE " + c.Where
	}
	if c.OrderBy != "" {
		dataSQL += " ORDER BY " + c.OrderBy
	}

	dataSQL += fmt.Sprintf(" LIMIT %d OFFSET %d", c.Limit, c.Offset)

	var items []T
	if err := sqlx.SelectContext(ctx, db, &items, dataSQL, c.Args...); err != nil {
		return goquery.PageResult[T]{}, err
	}

	return goquery.PageResult[T]{
		Items: items,
		Meta:  goquery.BuildPageMeta(total, spec.Page, spec.Limit),
	}, nil
}

func BuildFullQuery(baseQuery string, c Clauses) string {
	var sb strings.Builder
	sb.WriteString(baseQuery)

	if c.Where != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(c.Where)
	}
	if c.OrderBy != "" {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(c.OrderBy)
	}
	if c.Limit > 0 {
		fmt.Fprintf(&sb, " LIMIT %d", c.Limit)
	}
	if c.Offset > 0 {
		fmt.Fprintf(&sb, " OFFSET %d", c.Offset)
	}
	return sb.String()
}
