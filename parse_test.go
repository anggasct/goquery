package goquery

import (
	"net/url"
	"testing"
)

func baseConfig() Config {
	return Config{
		AllowSearch:  []string{"companyName", "email"},
		AllowSort:    []string{"createdAt", "companyName"},
		AllowInclude: []string{"programs"},
		AllowFilter: map[string][]string{
			"status":        {"eq", "in"},
			"companyName":   {"eq", "like", "in"},
			"agreementDate": {"eq", "gte", "lte", "between"},
		},
		AllowFields: map[string][]string{
			"employer": {"id", "companyName", "status"},
			"programs": {"id", "programName"},
		},
		DefaultFieldEntity: "employer",
		DefaultPage:        1,
		DefaultLimit:       10,
		MaxLimit:           100,
	}
}

func TestParseValidQuery(t *testing.T) {
	raw := "status=active&filter[companyName][like]=abc&sort=-createdAt&include=programs&fields=id,companyName&q=foo&page=2&limit=10"
	values, _ := url.ParseQuery(raw)

	spec, err := Parse(values, baseConfig())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if spec.Page != 2 || spec.Limit != 10 {
		t.Fatalf("unexpected pagination: %+v", spec)
	}
	if spec.Q != "foo" {
		t.Fatalf("expected q=foo, got %s", spec.Q)
	}
	if len(spec.Sort) != 1 || !spec.Sort[0].Desc || spec.Sort[0].Field != "createdAt" {
		t.Fatalf("unexpected sort: %+v", spec.Sort)
	}
	if len(spec.Includes) != 1 || spec.Includes[0] != "programs" {
		t.Fatalf("unexpected include: %+v", spec.Includes)
	}
	if len(spec.Fields["employer"]) != 2 {
		t.Fatalf("unexpected fields: %+v", spec.Fields)
	}
	if len(spec.Filters) != 2 {
		t.Fatalf("expected 2 filters, got %+v", spec.Filters)
	}
}

func TestParseRejectsUnknownQueryParam(t *testing.T) {
	values, _ := url.ParseQuery("unknown=x")
	_, err := Parse(values, baseConfig())
	if err == nil {
		t.Fatal("expected error for unknown query key")
	}
}

func TestParseRejectsInvalidOperator(t *testing.T) {
	values, _ := url.ParseQuery("filter[status][between]=a:b")
	_, err := Parse(values, baseConfig())
	if err == nil {
		t.Fatal("expected invalid operator error")
	}
}

func TestParseRejectsNegativeLimit(t *testing.T) {
	values, _ := url.ParseQuery("limit=-1")
	_, err := Parse(values, baseConfig())
	if err == nil {
		t.Fatal("expected error for limit=-1 from query string")
	}
}

func TestBuildPageMeta(t *testing.T) {
	meta := BuildPageMeta(21, 2, 10)
	if meta.TotalPages != 3 {
		t.Fatalf("expected total pages 3, got %d", meta.TotalPages)
	}
	if !meta.HasNext || !meta.HasPrev {
		t.Fatalf("unexpected pagination flags: %+v", meta)
	}
}
