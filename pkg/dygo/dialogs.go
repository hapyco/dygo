package dygo

// DialogType controls the visual intent of a Studio dialog.
type DialogType string

const (
	DialogNeutral DialogType = "neutral"
	DialogInfo    DialogType = "info"
	DialogSuccess DialogType = "success"
	DialogWarning DialogType = "warning"
	DialogDanger  DialogType = "danger"
)

// DialogActionVariant controls how a Studio dialog action is styled.
type DialogActionVariant string

const (
	DialogActionPrimary   DialogActionVariant = "primary"
	DialogActionSecondary DialogActionVariant = "secondary"
	DialogActionDanger    DialogActionVariant = "danger"
)

// DialogAction is one user-selectable dialog action.
type DialogAction struct {
	Key     string              `json:"key"`
	Label   string              `json:"label"`
	Variant DialogActionVariant `json:"variant,omitempty"`
}

// Dialog is a server-provided Studio dialog intent.
type Dialog struct {
	Title       string         `json:"title"`
	Content     string         `json:"content,omitempty"`
	Type        DialogType     `json:"type,omitempty"`
	Actions     []DialogAction `json:"actions,omitempty"`
	Dismissible *bool          `json:"dismissible,omitempty"`
}
