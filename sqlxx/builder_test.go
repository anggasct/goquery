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

func TestBuildWithDefaultSort(t *testing.T) {
	spec := goquery.Spec{Page: 1, Limit: 10}
	opts := Options{
		Dialect:     goquery.DialectPtr(goquery.Postgres),
		DefaultSort: "created_at DESC",
	}

	c := Build(spec, opts)
	if c.OrderBy != "created_at DESC" {
		t.Fatalf("expected default sort, got %s", c.OrderBy)
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
