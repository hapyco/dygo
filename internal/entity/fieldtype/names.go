package fieldtype

import "regexp"

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

// IsName reports whether value is a valid dygo metadata name.
func IsName(value string) bool {
	return namePattern.MatchString(value)
}
