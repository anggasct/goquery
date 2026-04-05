# goquery

A Go library for building dynamic database queries with filtering, searching, sorting, pagination, and relation loading. Supports multiple ORMs (sqlx, GORM, Bun) and database dialects (PostgreSQL, MySQL).

## Features

- Parse URL query parameters into a structured query specification
- Config-driven validation — define allowed filters, sort fields, search fields, and includes per endpoint
- Support for multiple filter operators: `eq`, `in`, `like`, `gt`, `gte`, `lt`, `lte`, `between`
- Full-text search across multiple fields
- Multi-field sorting with ascending/descending support
- Pagination with metadata (total, totalPages, hasNext, hasPrev)
- Relation loading via `?include=` with ORM-specific adapters
- Dialect-aware SQL generation (PostgreSQL `ILIKE`, `$N` placeholders, `NULLS LAST`; MySQL `LIKE`, `?` placeholders)
- ORM adapters for **sqlx**, **GORM**, and **Bun**

## Installation

Core module:

```bash
go get github.com/anggasct/goquery
```

ORM adapters (install only what you need):

```bash
go get github.com/anggasct/goquery/sqlxx
go get github.com/anggasct/goquery/gormx
go get github.com/anggasct/goquery/bunx
```

## Usage

### Parsing URL Query Parameters

```go
cfg := goquery.Config{
    AllowSearch:  []string{"name", "email"},
    AllowSort:    []string{"name", "createdAt"},
    AllowFilter:  map[string][]string{
        "status": {"eq", "in"},
        "age":    {"eq", "gt", "gte", "lt", "lte", "between"},
    },
    AllowInclude: []string{"profile", "orders"},
    DefaultPage:  1,
    DefaultLimit: 10,
    MaxLimit:     100,
    DefaultSort:  "created_at DESC",
}

// Example: ?page=2&limit=10&q=john&sort=-createdAt&filter[status][in]=active,pending&include=profile
spec, err := goquery.Parse(r.URL.Query(), cfg)
```

### With sqlx

#### Basic pagination

```go
result, err := sqlxx.Paginate[User](ctx, db, spec, "users", sqlxx.Options{
    FieldToCol: map[string]string{"createdAt": "created_at"},
})
// result.Items = []User
// result.Meta  = goquery.PageMeta{Page, Limit, Total, TotalPages, HasNext, HasPrev}
```

Default sort precedence for `sqlxx`:
- If `spec.Sort` is present, explicit sort from query/programmatic spec is used.
- If `spec.Sort` is empty, `spec.DefaultSort` is used first (for example from `goquery.Parse` + `Config.DefaultSort`).
- If both are empty, `sqlxx.Options.DefaultSort` is used as fallback.

#### Scope, FilterScope, and Select

```go
result, err := sqlxx.Paginate[Article](ctx, db, spec, "article a", sqlxx.Options{
    Select: "a.*",
    Scope: func(w *sqlxx.WhereBuilder) {
        w.Add("a.deleted_at IS NULL")
    },
    FilterScope: map[string]sqlxx.FilterFunc{
        "tag": func(w *sqlxx.WhereBuilder, f goquery.Filter) {
            w.Add(`EXISTS (
                SELECT 1 FROM article_tag at
                JOIN tag t ON t.id = at.tag_id
                WHERE at.article_id = a.id AND t.slug = ?
            )`, f.Values[0])
        },
    },
    FieldToCol: map[string]string{
        "createdAt": "a.created_at",
        "status":    "a.status",
    },
})
```

#### Relation loading with PaginateWith

Use `PaginateWith` to batch-load relations based on `?include=` parameters, eliminating N+1 queries:

```go
result, err := sqlxx.PaginateWith[Article](ctx, db, spec, "article a",
    sqlxx.Options{
        Select: "a.*",
        Scope:  func(w *sqlxx.WhereBuilder) { w.Add("a.deleted_at IS NULL") },
    },
    map[string]sqlxx.IncludeFunc[Article]{
        "category": sqlxx.BelongsTo[Article, Category](
            "category", "category_id", "Category", "deleted_at IS NULL",
        ),
        "author": sqlxx.BelongsTo[Article, User](
            `"user"`, "author_id", "Author", "deleted_at IS NULL",
        ),
        "tags": sqlxx.HasMany[Article, Tag](
            "tag", "article_tag", "article_id", "tag_id", "Tags", "deleted_at IS NULL",
        ),
    },
)
```

**`BelongsTo[T, R](table, fkCol, field, conds...)`** — batch-loads a belongs-to relation:
- `table`: related table name
- `fkCol`: `db` tag of the FK field in T (handles nullable pointers)
- `field`: struct field name in T to assign the `*R` result
- `conds`: optional WHERE conditions

**`HasMany[T, R](relTable, joinTable, fkCol, relCol, field, conds...)`** — batch-loads a many-to-many relation via a join table:
- `relTable`: related table name
- `joinTable`: join table name
- `fkCol`: column in join table pointing to T
- `relCol`: column in join table pointing to R
- `field`: struct field name in T to assign the `[]R` result
- `conds`: optional WHERE conditions for the related table

Both use conventions: primary keys are resolved via the `db:"id"` struct tag, and fields are assigned by struct field name.

For custom include logic, implement `IncludeFunc[T]` directly:

```go
type IncludeFunc[T any] func(ctx context.Context, db *sqlx.DB, items []T) error
```

#### Build clauses manually

```go
opts := sqlxx.Options{
    Dialect:    goquery.DialectPtr(goquery.Postgres),
    FieldToCol: map[string]string{"createdAt": "created_at"},
}
clauses := sqlxx.Build(spec, opts)
fullSQL := sqlxx.BuildFullQuery("SELECT * FROM users", clauses)
// Execute with clauses.Args...
```

### With GORM

```go
opts := gormx.Options{
    FieldToCol: map[string]string{"createdAt": "created_at"},
    IncludeMap: map[string]func(*gorm.DB) *gorm.DB{
        "profile": func(db *gorm.DB) *gorm.DB {
            return db.Preload("Profile", "active = true")
        },
    },
}
result, err := gormx.Paginate[User](db.Model(&User{}), spec, opts)
```

Includes without a custom handler auto-map to `db.Preload(PascalCase(name))`.

### With Bun

```go
opts := bunx.Options{
    FieldToCol: map[string]string{"createdAt": "created_at"},
    Scope: func(q *bun.SelectQuery) *bun.SelectQuery {
        return q.Where("deleted_at IS NULL")
    },
    IncludeMap: map[string]func(*bun.SelectQuery) *bun.SelectQuery{
        "profile": func(q *bun.SelectQuery) *bun.SelectQuery {
            return q.Relation("Profile", func(q *bun.SelectQuery) *bun.SelectQuery {
                return q.Where("active = true")
            })
        },
    },
}
result, err := bunx.Paginate[User](ctx, db, spec, opts)
```

Includes without a custom handler auto-map to `q.Relation(PascalCase(name))`.

### Programmatic Filters

You can also build filters programmatically without URL parsing:

```go
spec := goquery.Spec{
    Page:  1,
    Limit: 20,
    Filters: []goquery.Filter{
        goquery.Eq("status", "active"),
        goquery.In("role", "admin", "editor"),
        goquery.Between("age", 18, 65),
        goquery.Gte("createdAt", "2024-01-01"),
        goquery.Like("name", "john"),
    },
    Sort: []goquery.SortField{
        {Field: "createdAt", Desc: true},
    },
    Includes: []string{"profile", "orders"},
}
```

## Query Parameter Reference

| Parameter | Format | Example |
|---|---|---|
| Pagination | `page=N&limit=N` | `?page=2&limit=20` |
| Search | `q=keyword` | `?q=john` |
| Sort | `sort=field` (asc), `sort=-field` (desc) | `?sort=-createdAt,name` |
| Include | `include=rel1,rel2` | `?include=profile,orders` |
| Fields | `fields=f1,f2` or `fields[entity]=f1,f2` | `?fields=id,name` |
| Filter (eq) | `field=value` or `filter[field]=value` | `?status=active` |
| Filter (operator) | `filter[field][op]=value` | `?filter[age][gte]=18` |
| Filter (in) | `filter[field][in]=a,b,c` | `?filter[status][in]=active,pending` |
| Filter (between) | `filter[field][between]=start:end` | `?filter[date][between]=2024-01-01:2024-12-31` |

## Filter Operators

| Operator | Description | Example |
|---|---|---|
| `eq` | Equal | `filter[status]=active` |
| `in` | In list | `filter[status][in]=a,b,c` |
| `like` | Pattern match (ILIKE on Postgres, LIKE on MySQL) | `filter[name][like]=john` |
| `gt` | Greater than | `filter[age][gt]=18` |
| `gte` | Greater than or equal | `filter[age][gte]=18` |
| `lt` | Less than | `filter[age][lt]=65` |
| `lte` | Less than or equal | `filter[age][lte]=65` |
| `between` | Range (inclusive) | `filter[age][between]=18:65` |

## Pagination Response

All `Paginate` functions return `goquery.PageResult[T]`:

```json
{
  "items": [...],
  "meta": {
    "page": 2,
    "limit": 10,
    "total": 95,
    "totalPages": 10,
    "hasNext": true,
    "hasPrev": true
  }
}
```
