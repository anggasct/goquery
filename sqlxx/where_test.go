package sqlxx

import (
	"testing"

	"github.com/anggasct/goquery"
)

func TestWhereBuilderEmpty(t *testing.T) {
	wb := &WhereBuilder{}
	if !wb.IsEmpty() {
		t.Fatal("expected empty")
	}
	parts, args := wb.resolve(goquery.Postgres, 0)
	if len(parts) != 0 || len(args) != 0 {
		t.Fatal("expected nil results for empty builder")
	}
}

func TestWhereBuilderNoArgs(t *testing.T) {
	wb := &WhereBuilder{}
	wb.Add("deleted_at IS NULL")

	parts, args := wb.resolve(goquery.Postgres, 0)
	if len(parts) != 1 || parts[0] != "deleted_at IS NULL" {
		t.Fatalf("unexpected parts: %v", parts)
	}
	if len(args) != 0 {
		t.Fatalf("expected 0 args, got %d", len(args))
	}
}

func TestWhereBuilderPostgres(t *testing.T) {
	wb := &WhereBuilder{}
	wb.Add("status = ?", "active")
	wb.Add("age BETWEEN ? AND ?", 18, 65)

	parts, args := wb.resolve(goquery.Postgres, 0)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[0] != "status = $1" {
		t.Fatalf("expected 'status = $1', got %q", parts[0])
	}
	if parts[1] != "age BETWEEN $2 AND $3" {
		t.Fatalf("expected 'age BETWEEN $2 AND $3', got %q", parts[1])
	}
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(args))
	}
}

func TestWhereBuilderPostgresWithOffset(t *testing.T) {
	wb := &WhereBuilder{}
	wb.Add("status = ?", "active")

	parts, args := wb.resolve(goquery.Postgres, 2)
	if parts[0] != "status = $3" {
		t.Fatalf("expected 'status = $3' with offset 2, got %q", parts[0])
	}
	if len(args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(args))
	}
}

func TestWhereBuilderMySQL(t *testing.T) {
	wb := &WhereBuilder{}
	wb.Add("status = ?", "active")
	wb.Add("age > ?", 18)

	parts, args := wb.resolve(goquery.MySQL, 0)
	if parts[0] != "status = ?" {
		t.Fatalf("expected MySQL passthrough, got %q", parts[0])
	}
	if parts[1] != "age > ?" {
		t.Fatalf("expected MySQL passthrough, got %q", parts[1])
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
}

func TestWhereBuilderSubquery(t *testing.T) {
	wb := &WhereBuilder{}
	wb.Add(`EXISTS (SELECT 1 FROM category c WHERE c.id = a.category_id AND c.slug = ?)`, "tech")

	parts, args := wb.resolve(goquery.Postgres, 0)
	expected := `EXISTS (SELECT 1 FROM category c WHERE c.id = a.category_id AND c.slug = $1)`
	if parts[0] != expected {
		t.Fatalf("expected %q, got %q", expected, parts[0])
	}
	if len(args) != 1 || args[0] != "tech" {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestWhereBuilderChaining(t *testing.T) {
	wb := &WhereBuilder{}
	wb.Add("a = ?", 1).Add("b = ?", 2).Add("c = ?", 3)

	if wb.IsEmpty() {
		t.Fatal("expected non-empty")
	}

	parts, args := wb.resolve(goquery.Postgres, 0)
	if len(parts) != 3 || len(args) != 3 {
		t.Fatalf("expected 3 parts and 3 args, got %d/%d", len(parts), len(args))
	}
	if parts[2] != "c = $3" {
		t.Fatalf("expected 'c = $3', got %q", parts[2])
	}
}

func TestBuildWithScope(t *testing.T) {
	spec := goquery.Spec{
		Page:         1,
		Limit:        10,
		Filters:      []goquery.Filter{{Field: "role", Operator: "eq", Values: []any{"admin"}}},
		SearchFields: []string{},
	}

	opts := Options{
		FieldToCol: map[string]string{"role": "u.role"},
		Dialect:    goquery.DialectPtr(goquery.Postgres),
		Scope: func(w *WhereBuilder) {
			w.Add("deleted_at IS NULL")
			w.Add("org_id = ?", "org-123")
		},
	}

	c := Build(spec, opts)

	// Scope: deleted_at IS NULL (no arg), org_id = $1
	// Filters: u.role = $2
	if len(c.Args) != 2 {
		t.Fatalf("expected 2 args, got %d: %v", len(c.Args), c.Args)
	}
	if c.Args[0] != "org-123" {
		t.Fatalf("expected first arg 'org-123', got %v", c.Args[0])
	}
	if c.Args[1] != "admin" {
		t.Fatalf("expected second arg 'admin', got %v", c.Args[1])
	}

	// WHERE should contain both scope and filter conditions
	if c.Where == "" {
		t.Fatal("expected non-empty WHERE")
	}
}

func TestBuildWithScopeNoFilters(t *testing.T) {
	spec := goquery.Spec{Page: 1, Limit: 10}

	opts := Options{
		Dialect: goquery.DialectPtr(goquery.Postgres),
		Scope: func(w *WhereBuilder) {
			w.Add("deleted_at IS NULL")
		},
	}

	c := Build(spec, opts)
	if c.Where != "deleted_at IS NULL" {
		t.Fatalf("expected 'deleted_at IS NULL', got %q", c.Where)
	}
	if len(c.Args) != 0 {
		t.Fatalf("expected 0 args, got %d", len(c.Args))
	}
}

func TestBuildWithNilScope(t *testing.T) {
	spec := goquery.Spec{Page: 1, Limit: 10}
	opts := Options{Dialect: goquery.DialectPtr(goquery.Postgres)}

	c := Build(spec, opts)
	if c.Where != "" {
		t.Fatalf("expected empty WHERE, got %q", c.Where)
	}
}
