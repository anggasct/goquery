package sqlxx

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/anggasct/goquery"
	"github.com/jmoiron/sqlx"
)

// IncludeFunc loads a relation for a batch of items in-place.
type IncludeFunc[T any] func(ctx context.Context, db *sqlx.DB, items []T) error

// ApplyIncludes runs the include handlers that match spec.Includes.
func ApplyIncludes[T any](ctx context.Context, db *sqlx.DB, spec goquery.Spec, items []T, includeMap map[string]IncludeFunc[T]) error {
	for _, inc := range spec.Includes {
		if fn, ok := includeMap[inc]; ok {
			if err := fn(ctx, db, items); err != nil {
				return err
			}
		}
	}
	return nil
}

// PaginateWith is like Paginate but also applies include handlers from spec.Includes.
func PaginateWith[T any](
	ctx context.Context,
	db *sqlx.DB,
	spec goquery.Spec,
	from string,
	opts Options,
	includeMap map[string]IncludeFunc[T],
) (goquery.PageResult[T], error) {
	result, err := Paginate[T](ctx, db, spec, from, opts)
	if err != nil {
		return result, err
	}
	if len(result.Items) > 0 && len(includeMap) > 0 {
		if err := ApplyIncludes(ctx, db, spec, result.Items, includeMap); err != nil {
			return result, err
		}
	}
	return result, nil
}

// BelongsTo returns an IncludeFunc that batch-loads a belongs-to relation.
//
// Convention: R's primary key is the field with db:"id" tag.
//
// Parameters:
//   - table: SQL table name (e.g. "category", `"user"`)
//   - fkCol: db tag of the FK field in T (e.g. "category_id"); handles nullable pointers
//   - field: struct field name in T to assign the *R result (e.g. "Category")
//   - conds: optional WHERE conditions (e.g. "deleted_at IS NULL")
//
// Example:
//
//	sqlxx.BelongsTo[model.Article, model.Category]("category", "category_id", "Category", "deleted_at IS NULL")
func BelongsTo[T any, R any](table, fkCol, field string, conds ...string) IncludeFunc[T] {
	tType := reflect.TypeOf((*T)(nil)).Elem()
	rType := reflect.TypeOf((*R)(nil)).Elem()
	fkIdx := mustFieldByDBTag(tType, fkCol)
	pkIdx := mustFieldByDBTag(rType, "id")
	setIdx := mustFieldByName(tType, field)

	where := "id IN (?)"
	for _, c := range conds {
		where += " AND " + c
	}
	queryTpl := fmt.Sprintf("SELECT * FROM %s WHERE %s", table, where)

	return func(ctx context.Context, db *sqlx.DB, items []T) error {
		seen := make(map[any]bool)
		var keys []any
		for i := range items {
			k, ok := reflectKey(reflect.ValueOf(&items[i]).Elem().Field(fkIdx))
			if !ok || seen[k] {
				continue
			}
			seen[k] = true
			keys = append(keys, k)
		}
		if len(keys) == 0 {
			return nil
		}

		q, args, err := sqlx.In(queryTpl, keys)
		if err != nil {
			return err
		}

		var results []R
		if err := sqlx.SelectContext(ctx, db, &results, db.Rebind(q), args...); err != nil {
			return err
		}

		m := make(map[any]*R, len(results))
		for i := range results {
			pk := reflect.ValueOf(&results[i]).Elem().Field(pkIdx).Interface()
			m[pk] = &results[i]
		}

		for i := range items {
			v := reflect.ValueOf(&items[i]).Elem()
			k, ok := reflectKey(v.Field(fkIdx))
			if ok {
				if r, exists := m[k]; exists {
					v.Field(setIdx).Set(reflect.ValueOf(r))
				}
			}
		}
		return nil
	}
}

// HasMany returns an IncludeFunc that batch-loads a many-to-many relation via a join table.
//
// Convention: both T's and R's primary key is the field with db:"id" tag.
//
// Parameters:
//   - relTable: related table name (e.g. "tag")
//   - joinTable: join table name (e.g. "article_tag")
//   - fkCol: column in join table pointing to T (e.g. "article_id")
//   - relCol: column in join table pointing to R (e.g. "tag_id")
//   - field: struct field name in T to assign the []R result (e.g. "Tags")
//   - conds: optional WHERE conditions for the related table (e.g. "deleted_at IS NULL")
//
// Example:
//
//	sqlxx.HasMany[model.Article, model.Tag]("tag", "article_tag", "article_id", "tag_id", "Tags", "deleted_at IS NULL")
func HasMany[T any, R any](relTable, joinTable, fkCol, relCol, field string, conds ...string) IncludeFunc[T] {
	tType := reflect.TypeOf((*T)(nil)).Elem()
	rType := reflect.TypeOf((*R)(nil)).Elem()
	tIDIdx := mustFieldByDBTag(tType, "id")
	setIdx := mustFieldByName(tType, field)

	// Build dynamic row type: struct { R (embedded); ParentKey__ <T.ID type> `db:"__pkey"` }
	rowType := reflect.StructOf([]reflect.StructField{
		{Name: rType.Name(), Type: rType, Anonymous: true},
		{Name: "ParentKey__", Type: tType.Field(tIDIdx).Type, Tag: reflect.StructTag(`db:"__pkey"`)},
	})

	// Build query template
	whereParts := []string{fmt.Sprintf("j.%s IN (?)", fkCol)}
	for _, c := range conds {
		whereParts = append(whereParts, "r."+c)
	}
	queryTpl := fmt.Sprintf(
		"SELECT r.*, j.%s AS __pkey FROM %s r JOIN %s j ON j.%s = r.id WHERE %s",
		fkCol, relTable, joinTable, relCol, strings.Join(whereParts, " AND "),
	)

	return func(ctx context.Context, db *sqlx.DB, items []T) error {
		var keys []any
		for i := range items {
			keys = append(keys, reflect.ValueOf(&items[i]).Elem().Field(tIDIdx).Interface())
		}

		q, args, err := sqlx.In(queryTpl, keys)
		if err != nil {
			return err
		}

		// Scan into dynamic slice via reflect
		slicePtr := reflect.New(reflect.SliceOf(rowType))
		if err := sqlx.SelectContext(ctx, db, slicePtr.Interface(), db.Rebind(q), args...); err != nil {
			return err
		}

		// Group by parent key
		rows := slicePtr.Elem()
		grouped := make(map[any][]reflect.Value)
		for i := 0; i < rows.Len(); i++ {
			row := rows.Index(i)
			parentKey := row.Field(1).Interface() // ParentKey__ field
			rVal := row.Field(0)                  // embedded R
			grouped[parentKey] = append(grouped[parentKey], rVal)
		}

		// Assign slices to items
		for i := range items {
			parentKey := reflect.ValueOf(&items[i]).Elem().Field(tIDIdx).Interface()
			rVals := grouped[parentKey]
			if len(rVals) == 0 {
				continue
			}
			slice := reflect.MakeSlice(reflect.SliceOf(rType), len(rVals), len(rVals))
			for j, rv := range rVals {
				slice.Index(j).Set(rv)
			}
			reflect.ValueOf(&items[i]).Elem().Field(setIdx).Set(slice)
		}
		return nil
	}
}

func reflectKey(v reflect.Value) (any, bool) {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil, false
		}
		return v.Elem().Interface(), true
	}
	return v.Interface(), true
}

func mustFieldByDBTag(t reflect.Type, tag string) int {
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Tag.Get("db") == tag {
			return i
		}
	}
	panic(fmt.Sprintf("sqlxx: type %s has no field with db:%q tag", t, tag))
}

func mustFieldByName(t reflect.Type, name string) int {
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Name == name {
			return i
		}
	}
	panic(fmt.Sprintf("sqlxx: type %s has no field named %q", t, name))
}
