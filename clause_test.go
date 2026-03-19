package goquery

import (
	"testing"
)

func TestClauseBuilderSearchPostgres(t *testing.T) {
	b := &ClauseBuilder{
		Dialect:    Postgres,
		FieldToCol: map[string]string{"name": "u.name", "email": "u.email"},
	}
	spec := Spec{Q: "foo", SearchFields: []string{"name", "email"}}

	clause := b.SearchClause(spec, 0)
	if clause.SQL != "(u.name ILIKE $1 OR u.email ILIKE $2)" {
		t.Fatalf("unexpected SQL: %s", clause.SQL)
	}
	if len(clause.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(clause.Args))
	}
	if clause.Args[0] != "%foo%" || clause.Args[1] != "%foo%" {
		t.Fatalf("unexpected args: %v", clause.Args)
	}
}

func TestClauseBuilderSearchMySQL(t *testing.T) {
	b := &ClauseBuilder{
		Dialect:    MySQL,
		FieldToCol: map[string]string{"name": "u.name"},
	}
	spec := Spec{Q: "bar", SearchFields: []string{"name"}}

	clause := b.SearchClause(spec, 0)
	if clause.SQL != "(u.name LIKE ?)" {
		t.Fatalf("unexpected SQL: %s", clause.SQL)
	}
}

func TestClauseBuilderSearchEmpty(t *testing.T) {
	b := &ClauseBuilder{Dialect: Postgres}
	spec := Spec{Q: "", SearchFields: []string{"name"}}

	clause := b.SearchClause(spec, 0)
	if clause.SQL != "" {
		t.Fatalf("expected empty SQL, got %s", clause.SQL)
	}
}

func TestClauseBuilderFilterClauses(t *testing.T) {
	b := &ClauseBuilder{
		Dialect:    Postgres,
		FieldToCol: map[string]string{"status": "t.status", "score": "t.score"},
	}
	spec := Spec{
		Filters: []Filter{
			{Field: "status", Operator: "eq", Values: []any{"active"}},
			{Field: "score", Operator: "gte", Values: []any{80}},
		},
	}

	clauses := b.FilterClauses(spec, 0)
	if len(clauses) != 2 {
		t.Fatalf("expected 2 clauses, got %d", len(clauses))
	}
	if clauses[0].SQL != "t.status = $1" {
		t.Fatalf("unexpected first clause: %s", clauses[0].SQL)
	}
	if clauses[1].SQL != "t.score >= $2" {
		t.Fatalf("unexpected second clause: %s", clauses[1].SQL)
	}
}

func TestClauseBuilderFilterIn(t *testing.T) {
	b := &ClauseBuilder{
		Dialect:    Postgres,
		FieldToCol: map[string]string{"status": "status"},
	}
	spec := Spec{
		Filters: []Filter{
			{Field: "status", Operator: "in", Values: []any{"a", "b", "c"}},
		},
	}

	clauses := b.FilterClauses(spec, 0)
	if len(clauses) != 1 {
		t.Fatalf("expected 1 clause, got %d", len(clauses))
	}
	if clauses[0].SQL != "status IN ($1, $2, $3)" {
		t.Fatalf("unexpected SQL: %s", clauses[0].SQL)
	}
	if len(clauses[0].Args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(clauses[0].Args))
	}
}

func TestClauseBuilderFilterBetween(t *testing.T) {
	b := &ClauseBuilder{
		Dialect:    MySQL,
		FieldToCol: map[string]string{"date": "created_at"},
	}
	spec := Spec{
		Filters: []Filter{
			{Field: "date", Operator: "between", Values: []any{"2024-01-01", "2024-12-31"}},
		},
	}

	clauses := b.FilterClauses(spec, 0)
	if clauses[0].SQL != "created_at BETWEEN ? AND ?" {
		t.Fatalf("unexpected SQL: %s", clauses[0].SQL)
	}
}

func TestClauseBuilderFilterLike(t *testing.T) {
	b := &ClauseBuilder{
		Dialect:    Postgres,
		FieldToCol: map[string]string{"name": "name"},
	}
	spec := Spec{
		Filters: []Filter{
			{Field: "name", Operator: "like", Values: []any{"john"}},
		},
	}

	clauses := b.FilterClauses(spec, 0)
	if clauses[0].SQL != "name ILIKE $1" {
		t.Fatalf("unexpected SQL: %s", clauses[0].SQL)
	}
	if clauses[0].Args[0] != "%john%" {
		t.Fatalf("unexpected arg: %v", clauses[0].Args[0])
	}
}

func TestClauseBuilderSortPostgres(t *testing.T) {
	b := &ClauseBuilder{
		Dialect:    Postgres,
		FieldToCol: map[string]string{"createdAt": "created_at", "name": "name"},
	}
	spec := Spec{
		Sort: []SortField{
			{Field: "createdAt", Desc: true},
			{Field: "name", Desc: false},
		},
	}

	sql := b.SortSQL(spec, "")
	if sql != "created_at DESC NULLS LAST, name ASC NULLS LAST" {
		t.Fatalf("unexpected SQL: %s", sql)
	}
}

func TestClauseBuilderSortMySQL(t *testing.T) {
	b := &ClauseBuilder{
		Dialect:    MySQL,
		FieldToCol: map[string]string{"createdAt": "created_at"},
	}
	spec := Spec{
		Sort: []SortField{
			{Field: "createdAt", Desc: true},
		},
	}

	sql := b.SortSQL(spec, "")
	if sql != "created_at DESC" {
		t.Fatalf("unexpected SQL: %s", sql)
	}
}

func TestClauseBuilderSortDefault(t *testing.T) {
	b := &ClauseBuilder{Dialect: Postgres}
	spec := Spec{}

	sql := b.SortSQL(spec, "created_at DESC")
	if sql != "created_at DESC" {
		t.Fatalf("unexpected SQL: %s", sql)
	}
}

func TestClauseBuilderPageLimitOffset(t *testing.T) {
	b := &ClauseBuilder{}
	spec := Spec{Page: 3, Limit: 10}

	limit, offset := b.PageLimitOffset(spec)
	if limit != 10 || offset != 20 {
		t.Fatalf("expected limit=10 offset=20, got limit=%d offset=%d", limit, offset)
	}
}

func TestClauseBuilderWhereClauses(t *testing.T) {
	b := &ClauseBuilder{
		Dialect:    Postgres,
		FieldToCol: map[string]string{"name": "name", "status": "status"},
	}
	spec := Spec{
		Q:            "foo",
		SearchFields: []string{"name"},
		Filters: []Filter{
			{Field: "status", Operator: "eq", Values: []any{"active"}},
		},
	}

	clause := b.WhereClauses(spec, 0)
	if clause.SQL != "(name ILIKE $1) AND status = $2" {
		t.Fatalf("unexpected SQL: %s", clause.SQL)
	}
	if len(clause.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(clause.Args))
	}
}

func TestResolveColumnNoMapping(t *testing.T) {
	b := &ClauseBuilder{}
	if col := b.ResolveColumn("name"); col != "name" {
		t.Fatalf("expected 'name', got '%s'", col)
	}
}

func TestResolveColumnWithMapping(t *testing.T) {
	b := &ClauseBuilder{FieldToCol: map[string]string{"name": "u.name"}}
	if col := b.ResolveColumn("name"); col != "u.name" {
		t.Fatalf("expected 'u.name', got '%s'", col)
	}
}

func TestResolveColumnEmpty(t *testing.T) {
	b := &ClauseBuilder{}
	if col := b.ResolveColumn(""); col != "" {
		t.Fatalf("expected empty, got '%s'", col)
	}
}
