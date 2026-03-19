package sqlxx

import (
	"context"
	"fmt"
	"strings"

	"github.com/anggasct/goquery"
	"github.com/jmoiron/sqlx"
)

type Options struct {
	FieldToCol  map[string]string
	DefaultSort string
	Dialect     *goquery.Dialect // nil = auto-detect from db connection
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

	where := b.WhereClauses(spec, 0)
	orderBy := b.SortSQL(spec, opts.DefaultSort)
	limit, offset := b.PageLimitOffset(spec)

	return Clauses{
		Where:   where.SQL,
		OrderBy: orderBy,
		Limit:   limit,
		Offset:  offset,
		Args:    where.Args,
	}
}

func Paginate[T any](
	ctx context.Context,
	db *sqlx.DB,
	spec goquery.Spec,
	baseQuery string,
	countQuery string,
	opts Options,
) (goquery.PageResult[T], error) {
	// Auto-detect dialect from db if not explicitly set
	if opts.Dialect == nil {
		d := resolveDialect(db, opts)
		opts.Dialect = &d
	}
	c := Build(spec, opts)

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
