// Package hookevents owns dygo's supported Record hook event catalog.
package hookevents

const (
	BeforeValidate = "before-validate"
	Validate       = "validate"
	BeforeCreate   = "before-create"
	AfterCreate    = "after-create"
	BeforeUpdate   = "before-update"
	AfterUpdate    = "after-update"
	BeforeDelete   = "before-delete"
	AfterDelete    = "after-delete"
)

// Spec describes one supported Record hook event.
type Spec struct {
	Name         string
	MutatesInput bool
}

var specs = []Spec{
	{Name: BeforeValidate, MutatesInput: true},
	{Name: Validate},
	{Name: BeforeCreate, MutatesInput: true},
	{Name: AfterCreate},
	{Name: BeforeUpdate, MutatesInput: true},
	{Name: AfterUpdate},
	{Name: BeforeDelete},
	{Name: AfterDelete},
}

// Specs returns supported events in stable lifecycle order.
func Specs() []Spec {
	cloned := make([]Spec, len(specs))
	copy(cloned, specs)
	return cloned
}

// Supported reports whether name is a supported hook event.
func Supported(name string) bool {
	for _, spec := range specs {
		if spec.Name == name {
			return true
		}
	}
	return false
}

// MutatesInput reports whether hooks for name can mutate target Record input.
func MutatesInput(name string) bool {
	for _, spec := range specs {
		if spec.Name == name {
			return spec.MutatesInput
		}
	}
	return false
}
