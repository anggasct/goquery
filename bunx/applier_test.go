package bunx

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/anggasct/goquery"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func testDB(t *testing.T) *bun.DB {
	t.Helper()
	sqldb, err := sql.Open(sqliteshim.ShimName, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	return bun.NewDB(sqldb, sqlitedialect.New())
}

type testModel struct {
	bun.BaseModel `bun:"table:users"`
	ID            int    `bun:"id,pk"`
	Name          string `bun:"name"`
	Email         string `bun:"email"`
	Status        string `bun:"status"`
}

func TestApplySearch(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	spec := goquery.Spec{
		Q:            "foo",
		SearchFields: []string{"name", "email"},
	}
	opts := Options{
		FieldToCol: map[string]string{"name": "name", "email": "email"},
		Dialect:    goquery.DialectPtr(goquery.Postgres),
	}

	q := db.NewSelect().Model((*testModel)(nil))
	q = ApplySearch(q, spec, opts)

	got := q.String()
	if !strings.Contains(got, "ILIKE") {
		t.Fatalf("expected ILIKE for Postgres, got: %s", got)
	}
	t.Log("ApplySearch SQL:", got)
}

func TestApplySearchMySQL(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	spec := goquery.Spec{
		Q:            "foo",
		SearchFields: []string{"name"},
	}
	opts := Options{
		FieldToCol: map[string]string{"name": "name"},
		Dialect:    goquery.DialectPtr(goquery.MySQL),
	}

	q := db.NewSelect().Model((*testModel)(nil))
	q = ApplySearch(q, spec, opts)

	got := q.String()
	if strings.Contains(got, "ILIKE") {
		t.Fatalf("expected LIKE (not ILIKE) for MySQL, got: %s", got)
	}
	if !strings.Contains(got, "LIKE") {
		t.Fatalf("expected LIKE for MySQL, got: %s", got)
	}
	t.Log("ApplySearch MySQL SQL:", got)
}

func TestApplySearchEmpty(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	spec := goquery.Spec{Q: "", SearchFields: []string{"name"}}

	q := db.NewSelect().Model((*testModel)(nil))
	before := q.String()
	q = ApplySearch(q, spec, Options{})
	after := q.String()

	if before != after {
		t.Fatal("expected query unchanged for empty search")
	}
}

func TestApplyFilters(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	spec := goquery.Spec{
		Filters: []goquery.Filter{
			{Field: "status", Operator: "eq", Values: []any{"active"}},
			{Field: "name", Operator: "like", Values: []any{"john"}},
		},
	}
	opts := Options{
		FieldToCol: map[string]string{"status": "status", "name": "name"},
		Dialect:    goquery.DialectPtr(goquery.Postgres),
	}

	q := db.NewSelect().Model((*testModel)(nil))
	q = ApplyFilters(q, spec, opts)

	got := q.String()
	if got == "" {
		t.Fatal("expected non-empty query")
	}
	t.Log("ApplyFilters SQL:", got)
}

func TestApplyFiltersIn(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	spec := goquery.Spec{
		Filters: []goquery.Filter{
			{Field: "status", Operator: "in", Values: []any{"active", "pending"}},
		},
	}
	opts := Options{FieldToCol: map[string]string{"status": "status"}}

	q := db.NewSelect().Model((*testModel)(nil))
	q = ApplyFilters(q, spec, opts)

	got := q.String()
	if got == "" {
		t.Fatal("expected non-empty query")
	}
	t.Log("ApplyFilters IN SQL:", got)
}

func TestApplyFiltersBetween(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	spec := goquery.Spec{
		Filters: []goquery.Filter{
			{Field: "id", Operator: "between", Values: []any{1, 100}},
		},
	}
	opts := Options{FieldToCol: map[string]string{"id": "id"}}

	q := db.NewSelect().Model((*testModel)(nil))
	q = ApplyFilters(q, spec, opts)

	got := q.String()
	if got == "" {
		t.Fatal("expected non-empty query")
	}
	t.Log("ApplyFilters BETWEEN SQL:", got)
}

func TestApplySortPostgres(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	spec := goquery.Spec{
		Sort: []goquery.SortField{
			{Field: "name", Desc: false},
			{Field: "id", Desc: true},
		},
	}
	opts := Options{
		FieldToCol: map[string]string{"name": "name", "id": "id"},
		Dialect:    goquery.DialectPtr(goquery.Postgres),
	}

	q := db.NewSelect().Model((*testModel)(nil))
	q = ApplySort(q, spec, opts)

	got := q.String()
	if !strings.Contains(got, "NULLS LAST") {
		t.Fatalf("expected NULLS LAST for Postgres, got: %s", got)
	}
	t.Log("ApplySort Postgres SQL:", got)
}

func TestApplySortMySQL(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	spec := goquery.Spec{
		Sort: []goquery.SortField{
			{Field: "name", Desc: false},
		},
	}
	opts := Options{
		FieldToCol: map[string]string{"name": "name"},
		Dialect:    goquery.DialectPtr(goquery.MySQL),
	}

	q := db.NewSelect().Model((*testModel)(nil))
	q = ApplySort(q, spec, opts)

	got := q.String()
	if strings.Contains(got, "NULLS LAST") {
		t.Fatalf("expected no NULLS LAST for MySQL, got: %s", got)
	}
	t.Log("ApplySort MySQL SQL:", got)
}

func TestApplySortDefault(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	spec := goquery.Spec{}
	opts := Options{DefaultSort: "created_at DESC"}

	q := db.NewSelect().Model((*testModel)(nil))
	q = ApplySort(q, spec, opts)

	got := q.String()
	if got == "" {
		t.Fatal("expected non-empty query")
	}
	t.Log("ApplySort default SQL:", got)
}

func TestApplyPage(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	spec := goquery.Spec{Page: 3, Limit: 20}

	q := db.NewSelect().Model((*testModel)(nil))
	q = ApplyPage(q, spec)

	got := q.String()
	if got == "" {
		t.Fatal("expected non-empty query")
	}
	t.Log("ApplyPage SQL:", got)
}

func TestApplyPageNoPage(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	spec := goquery.Spec{Page: 0, Limit: 10}

	q := db.NewSelect().Model((*testModel)(nil))
	before := q.String()
	q = ApplyPage(q, spec)
	after := q.String()

	if before != after {
		t.Fatal("expected query unchanged when page <= 0")
	}
}

func TestBuildSelectColumns(t *testing.T) {
	colMap := map[string]string{
		"name":  "u.name",
		"email": "u.email",
		"id":    "u.id",
	}

	cols := BuildSelectColumns(
		[]string{"name", "email"},
		colMap,
		[]string{"id"},
	)

	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %d: %v", len(cols), cols)
	}
	if cols[0] != "u.id" {
		t.Fatalf("expected first column u.id, got %s", cols[0])
	}
}

func TestBuildSelectColumnsDeduplicate(t *testing.T) {
	colMap := map[string]string{
		"id":   "u.id",
		"name": "u.name",
	}

	cols := BuildSelectColumns(
		[]string{"id", "name"},
		colMap,
		[]string{"id"},
	)

	if len(cols) != 2 {
		t.Fatalf("expected 2 columns (deduplicated), got %d: %v", len(cols), cols)
	}
}
