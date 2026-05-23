package recordquery

import (
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	params, err := Normalize(Params{})
	if err != nil {
		t.Fatalf("Normalize() error = %v, want nil", err)
	}
	if params.Limit != DefaultLimit || params.Offset != 0 {
		t.Fatalf("Normalize() = %+v, want default limit and zero offset", params)
	}

	if _, err := Normalize(Params{Limit: MaxLimit + 1}); err == nil {
		t.Fatal("Normalize(over max) error = nil, want error")
	}
	if _, err := Normalize(Params{Limit: 10, Offset: -1}); err == nil {
		t.Fatal("Normalize(negative offset) error = nil, want error")
	}
}

func TestListPolicy(t *testing.T) {
	policy := ListPolicy()
	if policy.DefaultLimit != 20 || policy.MaxLimit != 2500 {
		t.Fatalf("ListPolicy() = %+v, want default 20 max 2500", policy)
	}
	wantPageSizes := []int{20, 100, 500, 2500}
	if !reflect.DeepEqual(policy.PageSizes, wantPageSizes) {
		t.Fatalf("ListPolicy().PageSizes = %#v, want %#v", policy.PageSizes, wantPageSizes)
	}
}

func TestFromValues(t *testing.T) {
	values := url.Values{
		"limit":   {"25"},
		"offset":  {"5"},
		"status":  {"Open"},
		"enabled": {"true"},
		"sort":    {"-created-at,name"},
	}

	params, err := FromValues(values)
	if err != nil {
		t.Fatalf("FromValues() error = %v, want nil", err)
	}
	wantFilters := []Filter{{Field: "enabled", Value: "true"}, {Field: "status", Value: "Open"}}
	wantSort := []Sort{{Field: "created-at", Desc: true}, {Field: "name"}}
	if params.Limit != 25 || params.Offset != 5 {
		t.Fatalf("FromValues() pagination = %+v, want limit 25 offset 5", params)
	}
	if !reflect.DeepEqual(params.Filters, wantFilters) {
		t.Fatalf("FromValues() filters = %#v, want %#v", params.Filters, wantFilters)
	}
	if !reflect.DeepEqual(params.Sort, wantSort) {
		t.Fatalf("FromValues() sort = %#v, want %#v", params.Sort, wantSort)
	}
}

func TestFromValuesRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		values url.Values
		want   string
	}{
		{name: "limit not integer", values: url.Values{"limit": {"nope"}}, want: "limit must be an integer"},
		{name: "limit too high", values: url.Values{"limit": {"2501"}}, want: "limit must be between 1 and 2500"},
		{name: "offset not integer", values: url.Values{"offset": {"nope"}}, want: "offset must be an integer"},
		{name: "sort repeated", values: url.Values{"sort": {"name", "-created-at"}}, want: "sort must be provided once"},
		{name: "sort empty", values: url.Values{"sort": {"-"}}, want: "sort field is required"},
		{name: "filter repeated", values: url.Values{"status": {"Open", "Closed"}}, want: "filter field is duplicated"},
		{name: "reserved query", values: url.Values{"search": {"admin"}}, want: `query parameter "search" is reserved`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FromValues(tt.values)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("FromValues() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}
