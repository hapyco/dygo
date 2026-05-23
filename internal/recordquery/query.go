package recordquery

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const (
	DefaultLimit = 50
	MaxLimit     = 2500
)

// Params controls Record list pagination, exact filters, and sorting.
type Params struct {
	Limit   int
	Offset  int
	Filters []Filter
	Sort    []Sort
}

// Filter is one exact Record list filter.
type Filter struct {
	Field string
	Value string
}

// Sort is one Record list sort term.
type Sort struct {
	Field string
	Desc  bool
}

// Error reports stable Record query contract failures.
type Error struct {
	Message string
	Details map[string]any
	Err     error
}

func (e Error) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return e.Message + ": " + e.Err.Error()
}

func (e Error) Unwrap() error {
	return e.Err
}

// Normalize applies default pagination and validates pagination bounds.
func Normalize(params Params) (Params, error) {
	if params.Limit == 0 {
		params.Limit = DefaultLimit
	}
	if params.Limit < 1 || params.Limit > MaxLimit {
		return Params{}, Error{Message: fmt.Sprintf("limit must be between 1 and %d", MaxLimit), Details: map[string]any{"limit": params.Limit}}
	}
	if params.Offset < 0 {
		return Params{}, Error{Message: "offset must be greater than or equal to 0", Details: map[string]any{"offset": params.Offset}}
	}
	return params, nil
}

// FromValues decodes HTTP query params into the shared Record list query contract.
func FromValues(values url.Values) (Params, error) {
	params, err := PaginationFromValues(values)
	if err != nil {
		return Params{}, err
	}
	for name, rawValues := range values {
		switch name {
		case "limit", "offset":
			continue
		case "sort":
			if len(rawValues) > 1 {
				return Params{}, Error{Message: "sort must be provided once", Details: map[string]any{"sort": rawValues}}
			}
			sortTerms, err := ParseSort(rawValues[0])
			if err != nil {
				return Params{}, err
			}
			params.Sort = sortTerms
		default:
			if len(rawValues) > 1 {
				return Params{}, Error{Message: "filter field is duplicated", Details: map[string]any{"field": name}}
			}
			params.Filters = append(params.Filters, Filter{Field: name, Value: rawValues[0]})
		}
	}
	SortFilters(params.Filters)
	return params, nil
}

// PaginationFromValues decodes only limit and offset from HTTP query params.
func PaginationFromValues(values url.Values) (Params, error) {
	params := Params{}
	if value := strings.TrimSpace(values.Get("limit")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Params{}, Error{Message: "limit must be an integer", Details: map[string]any{"limit": value}, Err: err}
		}
		if parsed < 1 || parsed > MaxLimit {
			return Params{}, Error{Message: fmt.Sprintf("limit must be between 1 and %d", MaxLimit), Details: map[string]any{"limit": parsed}}
		}
		params.Limit = parsed
	}
	if value := strings.TrimSpace(values.Get("offset")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Params{}, Error{Message: "offset must be an integer", Details: map[string]any{"offset": value}, Err: err}
		}
		params.Offset = parsed
	}
	return Normalize(params)
}

// ParseSort decodes the HTTP sort grammar: field,-field,other-field.
func ParseSort(raw string) ([]Sort, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, Error{Message: "sort field is required", Details: map[string]any{"sort": raw}}
	}
	parts := strings.Split(raw, ",")
	sortTerms := make([]Sort, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "-" {
			return nil, Error{Message: "sort field is required", Details: map[string]any{"sort": raw}}
		}
		desc := strings.HasPrefix(part, "-")
		if desc {
			part = strings.TrimSpace(strings.TrimPrefix(part, "-"))
			if part == "" {
				return nil, Error{Message: "sort field is required", Details: map[string]any{"sort": raw}}
			}
		}
		sortTerms = append(sortTerms, Sort{Field: part, Desc: desc})
	}
	return sortTerms, nil
}

// SortFilters orders filters by field for deterministic downstream behavior.
func SortFilters(filters []Filter) {
	sort.SliceStable(filters, func(i, j int) bool {
		return filters[i].Field < filters[j].Field
	})
}
