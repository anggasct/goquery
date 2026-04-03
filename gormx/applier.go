package gormx

import (
	"fmt"
	"strings"

	"github.com/anggasct/goquery"
	"gorm.io/gorm"
)

type Options struct {
	FieldToCol  map[string]string
	DefaultSort string
	Dialect     *goquery.Dialect                       // nil = auto-detect from db connection
	Scope       func(db *gorm.DB) *gorm.DB             // custom query modifications (JOINs, WHERE, etc.)
	IncludeMap  map[string]func(*gorm.DB) *gorm.DB     // custom include handlers (overrides auto PascalCase)
}

func resolveColumn(field string, fieldToColumn map[string]string) string {
	b := &goquery.ClauseBuilder{FieldToCol: fieldToColumn}
	return b.ResolveColumn(field)
}

func resolveDialect(db *gorm.DB, opts Options) goquery.Dialect {
	if opts.Dialect != nil {
		return *opts.Dialect
	}
	return goquery.DetectDialect(db.Dialector.Name())
}

func likeOp(dialect goquery.Dialect) string {
	if dialect == goquery.MySQL {
		return "LIKE"
	}
	return "ILIKE"
}

func ApplyScope(db *gorm.DB, opts Options) *gorm.DB {
	if opts.Scope != nil {
		db = opts.Scope(db)
	}
	return db
}

func ApplyInclude(db *gorm.DB, spec goquery.Spec, opts Options) *gorm.DB {
	for _, inc := range spec.Includes {
		if fn, ok := opts.IncludeMap[inc]; ok {
			db = fn(db)
		} else {
			db = db.Preload(goquery.ToPascalCase(inc))
		}
	}
	return db
}

func ApplySearch(db *gorm.DB, spec goquery.Spec, opts Options) *gorm.DB {
	keyword := strings.TrimSpace(spec.Q)
	if keyword == "" || len(spec.SearchFields) == 0 {
		return db
	}

	dialect := resolveDialect(db, opts)
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
		return db
	}
	return db.Where("("+strings.Join(clauses, " OR ")+")", args...)
}

func ApplyFilters(db *gorm.DB, spec goquery.Spec, opts Options) *gorm.DB {
	dialect := resolveDialect(db, opts)
	for _, f := range spec.Filters {
		col := resolveColumn(f.Field, opts.FieldToCol)
		if col == "" || len(f.Values) == 0 {
			continue
		}
		switch f.Operator {
		case "eq":
			db = db.Where(col+" = ?", f.Values[0])
		case "in":
			db = db.Where(col+" IN ?", f.Values)
		case "like":
			db = db.Where(col+" "+likeOp(dialect)+" ?", "%"+fmt.Sprint(f.Values[0])+"%")
		case "gt":
			db = db.Where(col+" > ?", f.Values[0])
		case "gte":
			db = db.Where(col+" >= ?", f.Values[0])
		case "lt":
			db = db.Where(col+" < ?", f.Values[0])
		case "lte":
			db = db.Where(col+" <= ?", f.Values[0])
		case "between":
			if len(f.Values) == 2 {
				db = db.Where(col+" BETWEEN ? AND ?", f.Values[0], f.Values[1])
			}
		}
	}
	return db
}

func ApplySort(db *gorm.DB, spec goquery.Spec, opts Options) *gorm.DB {
	if len(spec.Sort) == 0 {
		if strings.TrimSpace(opts.DefaultSort) != "" {
			return db.Order(opts.DefaultSort)
		}
		return db
	}
	dialect := resolveDialect(db, opts)
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
		db = db.Order(clause)
	}
	return db
}

func ApplyPage(db *gorm.DB, spec goquery.Spec) *gorm.DB {
	if spec.Limit <= 0 || spec.Page <= 0 {
		return db
	}
	offset := (spec.Page - 1) * spec.Limit
	return db.Offset(offset).Limit(spec.Limit)
}

func Paginate[T any](db *gorm.DB, spec goquery.Spec, opts Options) (goquery.PageResult[T], error) {
	var items []T
	sortOpts := Options{FieldToCol: opts.FieldToCol, DefaultSort: spec.DefaultSort, Dialect: opts.Dialect}

	if spec.Page <= 0 || spec.Limit == -1 {
		q := ApplyScope(db, opts)
		q = ApplyInclude(q, spec, opts)
		q = ApplySearch(q, spec, opts)
		q = ApplyFilters(q, spec, opts)
		q = ApplySort(q, spec, sortOpts)
		if spec.Limit > 0 {
			q = q.Limit(spec.Limit)
		}
		if err := q.Find(&items).Error; err != nil {
			return goquery.PageResult[T]{}, err
		}
		n := int64(len(items))
		return goquery.PageResult[T]{
			Items: items,
			Meta:  goquery.BuildPageMeta(n, 1, len(items)),
		}, nil
	}

	var total int64
	countQ := ApplyScope(db.Session(&gorm.Session{}), opts)
	countQ = ApplySearch(countQ, spec, opts)
	countQ = ApplyFilters(countQ, spec, opts)
	if err := countQ.Count(&total).Error; err != nil {
		return goquery.PageResult[T]{}, err
	}

	dataQ := ApplyScope(db.Session(&gorm.Session{}), opts)
	dataQ = ApplyInclude(dataQ, spec, opts)
	dataQ = ApplySearch(dataQ, spec, opts)
	dataQ = ApplyFilters(dataQ, spec, opts)
	dataQ = ApplySort(dataQ, spec, sortOpts)
	dataQ = ApplyPage(dataQ, spec)
	if err := dataQ.Find(&items).Error; err != nil {
		return goquery.PageResult[T]{}, err
	}
	return goquery.PageResult[T]{
		Items: items,
		Meta:  goquery.BuildPageMeta(total, spec.Page, spec.Limit),
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
