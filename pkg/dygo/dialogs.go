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

// ToastType controls the visual intent of a Studio toast.
type ToastType string

const (
	ToastInfo    ToastType = "info"
	ToastSuccess ToastType = "success"
	ToastWarning ToastType = "warning"
	ToastDanger  ToastType = "danger"
)

// Toast is a server-provided Studio toast intent.
type Toast struct {
	Title    string    `json:"title"`
	Content  string    `json:"content,omitempty"`
	Type     ToastType `json:"type,omitempty"`
	Duration *int      `json:"duration,omitempty"`
}
