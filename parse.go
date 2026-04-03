package goquery

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	filterKeyPattern = regexp.MustCompile(`^filter\[([^\]]+)\](?:\[([^\]]+)\])?$`)
	fieldsKeyPattern = regexp.MustCompile(`^fields\[([^\]]+)\]$`)
)

type Config struct {
	AllowSearch        []string
	AllowSort          []string
	AllowFilter        map[string][]string
	AllowInclude       []string
	AllowFields        map[string][]string
	DefaultFieldEntity string
	DefaultPage        int
	DefaultLimit       int
	MaxLimit           int
	DefaultSort        string
}

type SortField struct {
	Field string
	Desc  bool
}

type Filter struct {
	Field    string
	Operator string
	Values   []any
}

func Eq(field string, val any) Filter {
	return Filter{Field: field, Operator: "eq", Values: []any{val}}
}

func Gt(field string, val any) Filter {
	return Filter{Field: field, Operator: "gt", Values: []any{val}}
}

func Gte(field string, val any) Filter {
	return Filter{Field: field, Operator: "gte", Values: []any{val}}
}

func Lt(field string, val any) Filter {
	return Filter{Field: field, Operator: "lt", Values: []any{val}}
}

func Lte(field string, val any) Filter {
	return Filter{Field: field, Operator: "lte", Values: []any{val}}
}

func In(field string, vals ...any) Filter {
	return Filter{Field: field, Operator: "in", Values: vals}
}

func Like(field string, val string) Filter {
	return Filter{Field: field, Operator: "like", Values: []any{val}}
}

func Between(field string, from, to any) Filter {
	return Filter{Field: field, Operator: "between", Values: []any{from, to}}
}

type Spec struct {
	Page         int
	Limit        int
	Q            string
	Sort         []SortField
	Filters      []Filter
	Includes     []string
	Fields       map[string][]string
	SearchFields []string
	DefaultSort  string
}

func Parse(values url.Values, cfg Config) (Spec, error) {
	setDefaults(&cfg)

	if err := validateKnownKeys(values, cfg); err != nil {
		return Spec{}, err
	}

	page, err := parsePositiveInt(values.Get("page"), cfg.DefaultPage, "page")
	if err != nil {
		return Spec{}, err
	}
	limit, err := parsePositiveInt(values.Get("limit"), cfg.DefaultLimit, "limit")
	if err != nil {
		return Spec{}, err
	}
	if limit > cfg.MaxLimit {
		limit = cfg.MaxLimit
	}

	sorts, err := parseSort(values.Get("sort"), cfg.AllowSort)
	if err != nil {
		return Spec{}, err
	}

	includes, err := parseIncludes(values.Get("include"), cfg.AllowInclude)
	if err != nil {
		return Spec{}, err
	}

	fields, err := parseFields(values, cfg)
	if err != nil {
		return Spec{}, err
	}

	filters, err := parseFilters(values, cfg.AllowFilter)
	if err != nil {
		return Spec{}, err
	}

	q := strings.TrimSpace(values.Get("q"))
	searchFields := []string{}
	if q != "" {
		if len(cfg.AllowSearch) == 0 {
			return Spec{}, fmt.Errorf("q is not allowed for this endpoint")
		}
		searchFields = append(searchFields, cfg.AllowSearch...)
	}

	return Spec{
		Page:         page,
		Limit:        limit,
		Q:            q,
		Sort:         sorts,
		Filters:      filters,
		Includes:     includes,
		Fields:       fields,
		SearchFields: searchFields,
		DefaultSort:  cfg.DefaultSort,
	}, nil
}

func setDefaults(cfg *Config) {
	if cfg.DefaultPage <= 0 {
		cfg.DefaultPage = 1
	}
	if cfg.DefaultLimit <= 0 {
		cfg.DefaultLimit = 10
	}
	if cfg.MaxLimit <= 0 {
		cfg.MaxLimit = 100
	}
	if cfg.DefaultFieldEntity == "" {
		cfg.DefaultFieldEntity = "default"
	}
}

func validateKnownKeys(values url.Values, cfg Config) error {
	for key := range values {
		if filterKeyPattern.MatchString(key) {
			matches := filterKeyPattern.FindStringSubmatch(key)
			field := strings.TrimSpace(matches[1])
			if _, ok := cfg.AllowFilter[field]; !ok {
				return fmt.Errorf("filter field is not allowed: %s", field)
			}
		}
	}
	return nil
}

func parsePositiveInt(raw string, fallback int, key string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return v, nil
}

func parseSort(raw string, allowSort []string) ([]SortField, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	allowed := makeSet(allowSort)
	parts := strings.Split(raw, ",")
	out := make([]SortField, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		desc := strings.HasPrefix(p, "-")
		field := p
		if desc {
			field = strings.TrimPrefix(p, "-")
		}
		if !allowed[field] {
			return nil, fmt.Errorf("sort field is not allowed: %s", field)
		}
		out = append(out, SortField{Field: field, Desc: desc})
	}
	return out, nil
}

func parseIncludes(raw string, allowInclude []string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	allowed := makeSet(allowInclude)
	parts := splitCSV(raw)
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, inc := range parts {
		if !allowed[inc] {
			return nil, fmt.Errorf("include is not allowed: %s", inc)
		}
		if seen[inc] {
			continue
		}
		seen[inc] = true
		out = append(out, inc)
	}
	return out, nil
}

func parseFields(values url.Values, cfg Config) (map[string][]string, error) {
	fields := map[string][]string{}
	for key, rawValues := range values {
		var entity string
		if key == "fields" {
			entity = cfg.DefaultFieldEntity
		} else {
			matches := fieldsKeyPattern.FindStringSubmatch(key)
			if len(matches) != 2 {
				continue
			}
			entity = strings.TrimSpace(matches[1])
		}

		allowedFields, ok := cfg.AllowFields[entity]
		if !ok {
			return nil, fmt.Errorf("fields target is not allowed: %s", entity)
		}

		selected, err := parseFieldsCSV(rawValues, allowedFields, entity)
		if err != nil {
			return nil, err
		}
		if len(selected) > 0 {
			fields[entity] = selected
		}
	}

	return fields, nil
}

func parseFieldsCSV(rawValues []string, allowedFields []string, entity string) ([]string, error) {
	allowed := makeSet(allowedFields)
	seen := map[string]bool{}
	out := make([]string, 0)
	for _, raw := range rawValues {
		for _, field := range splitCSV(raw) {
			if !allowed[field] {
				return nil, fmt.Errorf("field is not allowed for %s: %s", entity, field)
			}
			if seen[field] {
				continue
			}
			seen[field] = true
			out = append(out, field)
		}
	}
	return out, nil
}

func parseFilters(values url.Values, allowFilter map[string][]string) ([]Filter, error) {
	out := make([]Filter, 0)
	seen := map[string]bool{}

	for field := range allowFilter {
		direct := strings.TrimSpace(values.Get(field))
		if direct != "" {
			id := field + ":eq:" + direct
			if !seen[id] {
				seen[id] = true
				out = append(out, Filter{
					Field:    field,
					Operator: "eq",
					Values:   []any{direct},
				})
			}
		}
	}

	for key, rawValues := range values {
		matches := filterKeyPattern.FindStringSubmatch(key)
		if len(matches) == 0 {
			continue
		}
		field := strings.TrimSpace(matches[1])
		operator := "eq"
		if len(matches) == 3 && strings.TrimSpace(matches[2]) != "" {
			operator = strings.TrimSpace(matches[2])
		}
		allowedOps, ok := allowFilter[field]
		if !ok {
			return nil, fmt.Errorf("filter field is not allowed: %s", field)
		}
		if !makeSet(allowedOps)[operator] {
			return nil, fmt.Errorf("operator %s is not allowed for field %s", operator, field)
		}

		parsedValues, err := normalizeFilterValues(operator, rawValues)
		if err != nil {
			return nil, fmt.Errorf("invalid filter %s[%s]: %w", field, operator, err)
		}
		id := field + ":" + operator + ":" + strings.Join(parsedValues, "|")
		if seen[id] {
			continue
		}
		seen[id] = true
		anyValues := make([]any, len(parsedValues))
		for i, v := range parsedValues {
			anyValues[i] = v
		}
		out = append(out, Filter{
			Field:    field,
			Operator: operator,
			Values:   anyValues,
		})
	}
	return out, nil
}

func normalizeFilterValues(operator string, rawValues []string) ([]string, error) {
	combined := make([]string, 0)
	for _, raw := range rawValues {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		switch operator {
		case "in":
			combined = append(combined, splitCSV(raw)...)
		case "between":
			parts := strings.Split(raw, ":")
			if len(parts) != 2 {
				return nil, fmt.Errorf("between must be start:end")
			}
			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[1])
			if left == "" || right == "" {
				return nil, fmt.Errorf("between bounds must be non-empty")
			}
			combined = append(combined, left, right)
		default:
			combined = append(combined, raw)
		}
	}
	if len(combined) == 0 {
		return nil, fmt.Errorf("value is required")
	}
	if operator == "between" && len(combined) != 2 {
		return nil, fmt.Errorf("between must contain exactly two values")
	}
	return combined, nil
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func makeSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, v := range values {
		out[strings.TrimSpace(v)] = true
	}
	return out
}
