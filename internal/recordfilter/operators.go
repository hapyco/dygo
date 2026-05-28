package recordfilter

import "github.com/hapyco/dygo/internal/entity/fieldtype"

const (
	OperatorEqual              = "eq"
	OperatorNotEqual           = "ne"
	OperatorContains           = "contains"
	OperatorNotContains        = "not-contains"
	OperatorGreaterThan        = "gt"
	OperatorGreaterThanOrEqual = "gte"
	OperatorLessThan           = "lt"
	OperatorLessThanOrEqual    = "lte"
	OperatorBefore             = "before"
	OperatorAfter              = "after"
	OperatorBetween            = "between"
	OperatorEmpty              = "empty"
	OperatorNotEmpty           = "not-empty"
)

const (
	ArityNone  = "none"
	ArityOne   = "one"
	ArityRange = "range"
)

// Operator describes a Record list filter operator exposed to clients.
type Operator struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Arity string `json:"arity"`
}

var operatorsByKey = map[string]Operator{
	OperatorEqual:              {Key: OperatorEqual, Label: "is", Arity: ArityOne},
	OperatorNotEqual:           {Key: OperatorNotEqual, Label: "is not", Arity: ArityOne},
	OperatorContains:           {Key: OperatorContains, Label: "contains", Arity: ArityOne},
	OperatorNotContains:        {Key: OperatorNotContains, Label: "does not contain", Arity: ArityOne},
	OperatorGreaterThan:        {Key: OperatorGreaterThan, Label: "greater than", Arity: ArityOne},
	OperatorGreaterThanOrEqual: {Key: OperatorGreaterThanOrEqual, Label: "greater than or equal to", Arity: ArityOne},
	OperatorLessThan:           {Key: OperatorLessThan, Label: "less than", Arity: ArityOne},
	OperatorLessThanOrEqual:    {Key: OperatorLessThanOrEqual, Label: "less than or equal to", Arity: ArityOne},
	OperatorBefore:             {Key: OperatorBefore, Label: "before", Arity: ArityOne},
	OperatorAfter:              {Key: OperatorAfter, Label: "after", Arity: ArityOne},
	OperatorBetween:            {Key: OperatorBetween, Label: "between", Arity: ArityRange},
	OperatorEmpty:              {Key: OperatorEmpty, Label: "is empty", Arity: ArityNone},
	OperatorNotEmpty:           {Key: OperatorNotEmpty, Label: "is not empty", Arity: ArityNone},
}

var profileByFieldType = map[string][]string{
	"link": {
		OperatorEqual,
		OperatorNotEqual,
		OperatorEmpty,
		OperatorNotEmpty,
	},
	"select": {
		OperatorEqual,
		OperatorNotEqual,
		OperatorEmpty,
		OperatorNotEmpty,
	},
	"boolean": {
		OperatorEqual,
		OperatorNotEqual,
		OperatorEmpty,
		OperatorNotEmpty,
	},
	"date": {
		OperatorEqual,
		OperatorBefore,
		OperatorAfter,
		OperatorBetween,
		OperatorEmpty,
		OperatorNotEmpty,
	},
	"datetime": {
		OperatorEqual,
		OperatorBefore,
		OperatorAfter,
		OperatorBetween,
		OperatorEmpty,
		OperatorNotEmpty,
	},
	"time": {
		OperatorEqual,
		OperatorBefore,
		OperatorAfter,
		OperatorBetween,
		OperatorEmpty,
		OperatorNotEmpty,
	},
}

var numericOperators = []string{
	OperatorEqual,
	OperatorNotEqual,
	OperatorGreaterThan,
	OperatorGreaterThanOrEqual,
	OperatorLessThan,
	OperatorLessThanOrEqual,
	OperatorBetween,
	OperatorEmpty,
	OperatorNotEmpty,
}

var textOperators = []string{
	OperatorEqual,
	OperatorNotEqual,
	OperatorContains,
	OperatorNotContains,
	OperatorEmpty,
	OperatorNotEmpty,
}

// OperatorsForFieldType returns the operators supported by a persisted field type.
func OperatorsForFieldType(fieldType string, valueKind string) []Operator {
	if keys, ok := profileByFieldType[fieldType]; ok {
		return operatorsForKeys(keys)
	}
	switch valueKind {
	case fieldtype.ValueString:
		return operatorsForKeys(textOperators)
	case fieldtype.ValueInteger, fieldtype.ValueNumber:
		return operatorsForKeys(numericOperators)
	default:
		return nil
	}
}

// Supports reports whether operator is valid for a persisted field type.
func Supports(fieldType string, valueKind string, operator string) bool {
	for _, candidate := range OperatorsForFieldType(fieldType, valueKind) {
		if candidate.Key == operator {
			return true
		}
	}
	return false
}

// Arity returns the value arity for operator.
func Arity(operator string) (string, bool) {
	value, ok := operatorsByKey[operator]
	return value.Arity, ok
}

// IsZeroArity reports whether operator does not accept a value.
func IsZeroArity(operator string) bool {
	arity, ok := Arity(operator)
	return ok && arity == ArityNone
}

func operatorsForKeys(keys []string) []Operator {
	operators := make([]Operator, 0, len(keys))
	for _, key := range keys {
		if operator, ok := operatorsByKey[key]; ok {
			operators = append(operators, operator)
		}
	}
	return operators
}
