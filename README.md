# goquery

A Go library for building dynamic database queries with filtering, searching, sorting, and pagination. Supports multiple ORMs (sqlx, GORM, Bun) and database dialects (PostgreSQL, MySQL).

## Features

- Parse URL query parameters into a structured query specification
- Config-driven validation — define allowed filters, sort fields, and search fields per endpoint
- Support for multiple filter operators: `eq`, `in`, `like`, `gt`, `gte`, `lt`, `lte`, `between`
- Full-text search across multiple fields
- Multi-field sorting with ascending/descending support
- Pagination with metadata (total, totalPages, hasNext, hasPrev)
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
import (
    "net/url"
    "github.com/anggasct/goquery"
)

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

// Example: ?page=2&limit=10&q=john&sort=-createdAt&filter[status][in]=active,pending
spec, err := goquery.Parse(r.URL.Query(), cfg)
if err != nil {
    // handle validation error
}
```

### With sqlx

```go
import (
    "github.com/anggasct/goquery/sqlxx"
)

opts := sqlxx.Options{
    FieldToCol: map[string]string{
        "createdAt": "created_at",
    },
}

// Option 1: Build clauses manually (requires Dialect to be set explicitly)
opts.Dialect = goquery.DialectPtr(goquery.Postgres)
clauses := sqlxx.Build(spec, opts)
fullSQL := sqlxx.BuildFullQuery("SELECT * FROM users", clauses)
// Execute with clauses.Args...

// Option 2: Paginate directly (dialect auto-detected from db connection)
result, err := sqlxx.Paginate[User](
    ctx, db, spec,
    "SELECT * FROM users",
    "SELECT COUNT(*) FROM users",
    sqlxx.Options{
        FieldToCol: map[string]string{"createdAt": "created_at"},
    },
)
// result.Items = []User
// result.Meta  = goquery.PageMeta{Page, Limit, Total, TotalPages, HasNext, HasPrev}
```

### With GORM

```go
import (
    "github.com/anggasct/goquery/gormx"
)

// Dialect is auto-detected from the GORM db connection
opts := gormx.Options{
    FieldToCol: map[string]string{
        "createdAt": "created_at",
    },
}

result, err := gormx.Paginate[User](db.Model(&User{}), spec, opts)
```

### With Bun

```go
import (
    "github.com/anggasct/goquery/bunx"
)

// Dialect is auto-detected from the Bun db connection
opts := bunx.Options{
    FieldToCol: map[string]string{
        "createdAt": "created_at",
    },
}

result, err := bunx.Paginate[User](ctx, db, func(q *bun.SelectQuery) *bun.SelectQuery {
    return q.Where("deleted_at IS NULL")
}, spec, opts)
```

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
