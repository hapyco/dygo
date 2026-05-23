package fieldtype

import "regexp"

const NamePattern = `^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`

var namePattern = regexp.MustCompile(NamePattern)

// IsName reports whether value is a valid dygo metadata name.
func IsName(value string) bool {
	return namePattern.MatchString(value)
}
