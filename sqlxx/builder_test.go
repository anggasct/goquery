package sqlxx

import (
	"testing"

	"github.com/anggasct/goquery"
)

func TestBuildBasicPostgres(t *testing.T) {
	spec := goquery.Spec{
		Page:         1,
		Limit:        10,
		Q:            "foo",
		SearchFields: []string{"name", "email"},
		Filters: []goquery.Filter{
			{Field: "status", Operator: "eq", Values: []any{"active"}},
		},
		Sort: []goquery.SortField{
			{Field: "createdAt", Desc: true},
		},
	}

	opts := Options{
		FieldToCol: map[string]string{
			"name":      "u.name",
			"email":     "u.email",
			"status":    "u.status",
			"createdAt": "u.created_at",
		},
		Dialect: goquery.DialectPtr(goquery.Postgres),
	}

	c := Build(spec, opts)

	if c.Where == "" {
		t.Fatal("expected WHERE clause")
	}
	if c.OrderBy == "" {
		t.Fatal("expected ORDER BY clause")
	}
	if c.Limit != 10 {
		t.Fatalf("expected limit=10, got %d", c.Limit)
	}
	if c.Offset != 0 {
		t.Fatalf("expected offset=0, got %d", c.Offset)
	}
	if len(c.Args) != 3 {
		t.Fatalf("expected 3 args, got %d: %v", len(c.Args), c.Args)
	}
}

func TestBuildPage2(t *testing.T) {
	spec := goquery.Spec{Page: 3, Limit: 20}
	opts := Options{Dialect: goquery.DialectPtr(goquery.Postgres)}

	c := Build(spec, opts)
	if c.Limit != 20 {
		t.Fatalf("expected limit=20, got %d", c.Limit)
	}
	if c.Offset != 40 {
		t.Fatalf("expected offset=40, got %d", c.Offset)
	}
}

func TestBuildNoFilters(t *testing.T) {
	spec := goquery.Spec{Page: 1, Limit: 10}
	opts := Options{Dialect: goquery.DialectPtr(goquery.Postgres)}

	c := Build(spec, opts)
	if c.Where != "" {
		t.Fatalf("expected empty WHERE, got %s", c.Where)
	}
	if len(c.Args) != 0 {
		t.Fatalf("expected 0 args, got %d", len(c.Args))
	}
}

func TestBuildWithDefaultSortFromSpec(t *testing.T) {
	spec := goquery.Spec{
		Page:        1,
		Limit:       10,
		DefaultSort: "created_at DESC",
	}
	opts := Options{
		Dialect:     goquery.DialectPtr(goquery.Postgres),
		DefaultSort: "created_at ASC",
	}

	c := Build(spec, opts)
	if c.OrderBy != "created_at DESC" {
		t.Fatalf("expected spec default sort, got %s", c.OrderBy)
	}
}

func TestBuildWithDefaultSortFallbackToOptions(t *testing.T) {
	spec := goquery.Spec{Page: 1, Limit: 10}
	opts := Options{
		Dialect:     goquery.DialectPtr(goquery.Postgres),
		DefaultSort: "created_at DESC",
	}

	c := Build(spec, opts)
	if c.OrderBy != "created_at DESC" {
		t.Fatalf("expected options default sort fallback, got %s", c.OrderBy)
	}
}

func TestBuildWithExplicitSortOverridesDefaults(t *testing.T) {
	spec := goquery.Spec{
		Page:        1,
		Limit:       10,
		DefaultSort: "created_at DESC",
		Sort: []goquery.SortField{
			{Field: "name", Desc: false},
		},
	}
	opts := Options{
		Dialect:     goquery.DialectPtr(goquery.MySQL),
		DefaultSort: "created_at ASC",
	}

	c := Build(spec, opts)
	if c.OrderBy != "name ASC" {
		t.Fatalf("expected explicit sort to override defaults, got %s", c.OrderBy)
	}
}

func TestBuildMySQL(t *testing.T) {
	spec := goquery.Spec{
		Page:         1,
		Limit:        10,
		Q:            "test",
		SearchFields: []string{"name"},
		Sort: []goquery.SortField{
			{Field: "name", Desc: false},
		},
	}
	opts := Options{
		FieldToCol: map[string]string{"name": "name"},
		Dialect:    goquery.DialectPtr(goquery.MySQL),
	}

	c := Build(spec, opts)
	// MySQL uses LIKE not ILIKE
	if c.Where != "(name LIKE ?)" {
		t.Fatalf("expected MySQL LIKE, got %s", c.Where)
	}
	// MySQL should NOT have NULLS LAST
	if c.OrderBy != "name ASC" {
		t.Fatalf("expected 'name ASC', got %s", c.OrderBy)
	}
}

func TestBuildFullQuery(t *testing.T) {
	c := Clauses{
		Where:   "status = $1",
		OrderBy: "created_at DESC",
		Limit:   10,
		Offset:  20,
		Args:    []any{"active"},
	}

	sql := BuildFullQuery("SELECT * FROM users", c)
	expected := "SELECT * FROM users WHERE status = $1 ORDER BY created_at DESC LIMIT 10 OFFSET 20"
	if sql != expected {
		t.Fatalf("unexpected SQL:\ngot:  %s\nwant: %s", sql, expected)
	}
}

func TestBuildFullQueryNoWhere(t *testing.T) {
	c := Clauses{
		OrderBy: "id ASC",
		Limit:   5,
	}

	sql := BuildFullQuery("SELECT * FROM items", c)
	expected := "SELECT * FROM items ORDER BY id ASC LIMIT 5"
	if sql != expected {
		t.Fatalf("unexpected SQL:\ngot:  %s\nwant: %s", sql, expected)
	}
}

func TestBuildWithFilterScope(t *testing.T) {
	spec := goquery.Spec{
		Page:  1,
		Limit: 10,
		Filters: []goquery.Filter{
			{Field: "tag", Operator: "eq", Values: []any{"golang"}},
		},
	}

	opts := Options{
		Dialect: goquery.DialectPtr(goquery.Postgres),
		Scope: func(w *WhereBuilder) {
			w.Add("deleted_at IS NULL")
		},
		FilterScope: map[string]FilterFunc{
			"tag": func(w *WhereBuilder, f goquery.Filter) {
				w.Add(`EXISTS (SELECT 1 FROM tag t WHERE t.slug = ?)`, f.Values[0])
			},
		},
	}

	c := Build(spec, opts)

	if len(c.Args) != 1 {
		t.Fatalf("expected 1 arg, got %d: %v", len(c.Args), c.Args)
	}
	if c.Args[0] != "golang" {
		t.Fatalf("expected arg 'golang', got %v", c.Args[0])
	}
	if c.Where == "" {
		t.Fatal("expected non-empty WHERE")
	}
}

func TestBuildWithFilterScopeMixed(t *testing.T) {
	spec := goquery.Spec{
		Page:  1,
		Limit: 10,
		Filters: []goquery.Filter{
			{Field: "tag", Operator: "eq", Values: []any{"golang"}},
			{Field: "status", Operator: "eq", Values: []any{"published"}},
		},
	}

	opts := Options{
		FieldToCol: map[string]string{"status": "a.status"},
		Dialect:    goquery.DialectPtr(goquery.Postgres),
		Scope: func(w *WhereBuilder) {
			w.Add("a.deleted_at IS NULL")
		},
		FilterScope: map[string]FilterFunc{
			"tag": func(w *WhereBuilder, f goquery.Filter) {
				w.Add(`EXISTS (SELECT 1 FROM tag t WHERE t.slug = ?)`, f.Values[0])
			},
		},
	}

	c := Build(spec, opts)

	// Args: "golang" from FilterScope, "published" from default filter
	if len(c.Args) != 2 {
		t.Fatalf("expected 2 args, got %d: %v", len(c.Args), c.Args)
	}
	if c.Args[0] != "golang" {
		t.Fatalf("expected first arg 'golang', got %v", c.Args[0])
	}
	if c.Args[1] != "published" {
		t.Fatalf("expected second arg 'published', got %v", c.Args[1])
	}
}

func TestBuildWithFilterScopeNil(t *testing.T) {
	spec := goquery.Spec{
		Page:  1,
		Limit: 10,
		Filters: []goquery.Filter{
			{Field: "status", Operator: "eq", Values: []any{"active"}},
		},
	}

	opts := Options{
		FieldToCol: map[string]string{"status": "u.status"},
		Dialect:    goquery.DialectPtr(goquery.Postgres),
	}

	c := Build(spec, opts)

	if len(c.Args) != 1 || c.Args[0] != "active" {
		t.Fatalf("expected args ['active'], got %v", c.Args)
	}
	if c.Where == "" {
		t.Fatal("expected non-empty WHERE")
	}
}
