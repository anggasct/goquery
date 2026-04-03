package bunx

import (
	"context"
	"fmt"
	"strings"

	"github.com/anggasct/goquery"
	"github.com/uptrace/bun"
)

type Options struct {
	FieldToCol  map[string]string
	DefaultSort string
	Dialect     *goquery.Dialect                                          // nil = auto-detect from db connection
	Scope       func(q *bun.SelectQuery) *bun.SelectQuery                // custom query modifications (JOINs, WHERE, etc.)
	IncludeMap  map[string]func(*bun.SelectQuery) *bun.SelectQuery       // custom include handlers (overrides auto PascalCase)
}

func resolveColumn(field string, fieldToColumn map[string]string) string {
	b := &goquery.ClauseBuilder{FieldToCol: fieldToColumn}
	return b.ResolveColumn(field)
}

func detectDialect(db *bun.DB) goquery.Dialect {
	return goquery.DetectDialect(db.Dialect().Name().String())
}

func dialectFromOpts(opts Options, fallback goquery.Dialect) goquery.Dialect {
	if opts.Dialect != nil {
		return *opts.Dialect
	}
	return fallback
}

func likeOp(dialect goquery.Dialect) string {
	if dialect == goquery.MySQL {
		return "LIKE"
	}
	return "ILIKE"
}

func ApplyScope(q *bun.SelectQuery, opts Options) *bun.SelectQuery {
	if opts.Scope != nil {
		q = opts.Scope(q)
	}
	return q
}

func ApplyInclude(q *bun.SelectQuery, spec goquery.Spec, opts Options) *bun.SelectQuery {
	for _, inc := range spec.Includes {
		if fn, ok := opts.IncludeMap[inc]; ok {
			q = fn(q)
		} else {
			q = q.Relation(goquery.ToPascalCase(inc))
		}
	}
	return q
}

func ApplySearch(q *bun.SelectQuery, spec goquery.Spec, opts Options) *bun.SelectQuery {
	keyword := strings.TrimSpace(spec.Q)
	if keyword == "" || len(spec.SearchFields) == 0 {
		return q
	}

	dialect := dialectFromOpts(opts, goquery.Postgres)
	clauses := make([]string, 0, len(spec.SearchFields))
	args := make([]interface{}, 0, len(spec.SearchFields))
	for _, field := range spec.SearchFields {
		col := resolveColumn(field, opts.FieldToCol)
		if col == "" {
			continue
		}
		clauses = append(clauses, fmt.Sprintf("%s %s ?", col, likeOp(dialect)))
		args = append(args, "%"+keyword+"%")
	}
	if len(clauses) == 0 {
		return q
	}
	return q.Where("("+strings.Join(clauses, " OR ")+")", args...)
}

func ApplyFilters(q *bun.SelectQuery, spec goquery.Spec, opts Options) *bun.SelectQuery {
	dialect := dialectFromOpts(opts, goquery.Postgres)
	for _, f := range spec.Filters {
		col := resolveColumn(f.Field, opts.FieldToCol)
		if col == "" || len(f.Values) == 0 {
			continue
		}
		switch f.Operator {
		case "eq":
			q = q.Where(col+" = ?", f.Values[0])
		case "in":
			q = q.Where(col+" IN (?)", bun.In(f.Values))
		case "like":
			q = q.Where(col+" "+likeOp(dialect)+" ?", "%"+fmt.Sprint(f.Values[0])+"%")
		case "gt":
			q = q.Where(col+" > ?", f.Values[0])
		case "gte":
			q = q.Where(col+" >= ?", f.Values[0])
		case "lt":
			q = q.Where(col+" < ?", f.Values[0])
		case "lte":
			q = q.Where(col+" <= ?", f.Values[0])
		case "between":
			if len(f.Values) == 2 {
				q = q.Where(col+" BETWEEN ? AND ?", f.Values[0], f.Values[1])
			}
		}
	}
	return q
}

func ApplySort(q *bun.SelectQuery, spec goquery.Spec, opts Options) *bun.SelectQuery {
	if len(spec.Sort) == 0 {
		if strings.TrimSpace(opts.DefaultSort) != "" {
			return q.OrderExpr(opts.DefaultSort)
		}
		return q
	}
	dialect := dialectFromOpts(opts, goquery.Postgres)
	for _, s := range spec.Sort {
		col := resolveColumn(s.Field, opts.FieldToCol)
		if col == "" {
			continue
		}
		dir := "ASC"
		if s.Desc {
			dir = "DESC"
		}
		clause := col + " " + dir
		if dialect == goquery.Postgres {
			clause += " NULLS LAST"
		}
		q = q.OrderExpr(clause)
	}
	return q
}

func ApplyPage(q *bun.SelectQuery, spec goquery.Spec) *bun.SelectQuery {
	if spec.Limit <= 0 || spec.Page <= 0 {
		return q
	}
	offset := (spec.Page - 1) * spec.Limit
	return q.Offset(offset).Limit(spec.Limit)
}

func Paginate[T any](
	ctx context.Context,
	db *bun.DB,
	spec goquery.Spec,
	opts Options,
) (goquery.PageResult[T], error) {
	var items []T

	// Auto-detect dialect from db if not explicitly set
	if opts.Dialect == nil {
		d := detectDialect(db)
		opts.Dialect = &d
	}

	sortOpts := Options{FieldToCol: opts.FieldToCol, DefaultSort: spec.DefaultSort, Dialect: opts.Dialect}

	if spec.Page <= 0 || spec.Limit == -1 {
		q := db.NewSelect().Model(&items)
		q = ApplyScope(q, opts)
		q = ApplyInclude(q, spec, opts)
		q = ApplySearch(q, spec, opts)
		q = ApplyFilters(q, spec, opts)
		q = ApplySort(q, spec, sortOpts)
		if spec.Limit > 0 {
			q = q.Limit(spec.Limit)
		}
		if err := q.Scan(ctx); err != nil {
			return goquery.PageResult[T]{}, err
		}
		n := int64(len(items))
		return goquery.PageResult[T]{
			Items: items,
			Meta:  goquery.BuildPageMeta(n, 1, len(items)),
		}, nil
	}

	countQ := db.NewSelect().Model((*T)(nil))
	countQ = ApplyScope(countQ, opts)
	countQ = ApplySearch(countQ, spec, opts)
	countQ = ApplyFilters(countQ, spec, opts)
	total, err := countQ.Count(ctx)
	if err != nil {
		return goquery.PageResult[T]{}, err
	}

	dataQ := db.NewSelect().Model(&items)
	dataQ = ApplyScope(dataQ, opts)
	dataQ = ApplyInclude(dataQ, spec, opts)
	dataQ = ApplySearch(dataQ, spec, opts)
	dataQ = ApplyFilters(dataQ, spec, opts)
	dataQ = ApplySort(dataQ, spec, sortOpts)
	dataQ = ApplyPage(dataQ, spec)
	if err := dataQ.Scan(ctx); err != nil {
		return goquery.PageResult[T]{}, err
	}

	return goquery.PageResult[T]{
		Items: items,
		Meta:  goquery.BuildPageMeta(int64(total), spec.Page, spec.Limit),
	}, nil
}

func BuildSelectColumns(fields []string, fieldToColumn map[string]string, required []string) []string {
	out := make([]string, 0, len(fields)+len(required))
	seen := map[string]bool{}
	for _, field := range required {
		col := resolveColumn(field, fieldToColumn)
		if col == "" || seen[col] {
			continue
		}
		seen[col] = true
		out = append(out, col)
	}
	for _, field := range fields {
		col := resolveColumn(field, fieldToColumn)
		if col == "" || seen[col] {
			continue
		}
		seen[col] = true
		out = append(out, col)
	}
	return out
}
