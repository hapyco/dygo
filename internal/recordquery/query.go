package recordquery

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/hapyco/dygo/internal/recordfilter"
	"github.com/hapyco/dygo/internal/reserved"
)

const (
	DefaultLimit = 20
	MaxLimit     = 2500
)

var pageSizes = []int{20, 100, 500, MaxLimit}

// Policy describes the framework-owned Record list pagination contract.
type Policy struct {
	DefaultLimit int   `json:"default-limit"`
	MaxLimit     int   `json:"max-limit"`
	PageSizes    []int `json:"page-sizes"`
}

// ListPolicy returns the current Record list pagination policy.
func ListPolicy() Policy {
	return Policy{
		DefaultLimit: DefaultLimit,
		MaxLimit:     MaxLimit,
		PageSizes:    append([]int(nil), pageSizes...),
	}
}

// Params controls Record list pagination, operator filters, and sorting.
type Params struct {
	Limit   int
	Offset  int
	Filters []Filter
	Sort    []Sort
}

// Filter is one operator-based Record list filter.
type Filter struct {
	Field    string
	Operator string
	Value    string
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
			if reserved.IsQuery(name) {
				return Params{}, Error{Message: fmt.Sprintf("query parameter %q is reserved", name), Details: map[string]any{"query": name}}
			}
			for _, rawValue := range rawValues {
				filter, err := ParseFilterParam(name, rawValue)
				if err != nil {
					return Params{}, err
				}
				params.Filters = append(params.Filters, filter)
			}
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

// ParseFilterParam decodes the HTTP filter grammar: field:operator=value.
func ParseFilterParam(name string, value string) (Filter, error) {
	name = strings.TrimSpace(name)
	value = strings.TrimSpace(value)
	if name == "" {
		return Filter{}, Error{Message: "filter field is required", Details: map[string]any{"filter": name}}
	}
	field, operator, ok := strings.Cut(name, ":")
	if !ok {
		return Filter{}, Error{Message: fmt.Sprintf("unknown query parameter %q", name), Details: map[string]any{"query": name}}
	}
	field = strings.TrimSpace(field)
	operator = strings.TrimSpace(operator)
	if field == "" {
		return Filter{}, Error{Message: "filter field is required", Details: map[string]any{"filter": name}}
	}
	if operator == "" {
		return Filter{}, Error{Message: "filter operator is required", Details: map[string]any{"field": field, "filter": name}}
	}
	if recordfilter.IsZeroArity(operator) {
		if value != "" {
			return Filter{}, Error{Message: "filter value is not supported by this operator", Details: map[string]any{"field": field, "operator": operator}}
		}
		return Filter{Field: field, Operator: operator}, nil
	}
	if value == "" {
		return Filter{}, Error{Message: "filter value is required", Details: map[string]any{"field": field, "operator": operator}}
	}
	return Filter{Field: field, Operator: operator, Value: value}, nil
}

// SortFilters orders filters by field, operator, and value for deterministic downstream behavior.
func SortFilters(filters []Filter) {
	sort.SliceStable(filters, func(i, j int) bool {
		if filters[i].Field != filters[j].Field {
			return filters[i].Field < filters[j].Field
		}
		if filters[i].Operator != filters[j].Operator {
			return filters[i].Operator < filters[j].Operator
		}
		return filters[i].Value < filters[j].Value
	})
}
