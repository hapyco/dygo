# Dialogs

Status: v1 implementation decision log.

Studio needs one shared dialog surface for framework flows, metadata-driven actions, and app code that needs user confirmation.

## Decided

- Use `dialog` as the public Studio term.
- Use `dygo.Dialog` as the public SDK type name.
- Do not use `dygo.ThrowDialog` or `dygo.ShowDialog` as SDK type names.
- Treat `popup` as an informal word, not an API name.
- Dialogs are part of Studio shell, not individual pages.
- Studio exposes one dialog API that pages, stores, and framework flows can call.
- Use `useDialog()` as the frontend API name.
- `useDialog()` exposes `open`, `alert`, `confirm`, and `choose`.
- The first API should cover alert, confirm, and choice dialogs.
- The API returns the selected action instead of mutating caller state directly.
- Dialog actions are explicit objects with `key`, `label`, and optional `variant`.
- Dialog actions do not execute methods directly.
- Dialog actions return a selected `key`.
- Client-created dialog callers decide what returned action keys mean.
- Server-driven dialog action keys are sent back to the server with the confirmation token.
- Do not put method names, callback names, or dotted action paths in dialog actions in v1.
- Common action keys such as `ok`, `cancel`, `confirm`, `delete`, `retry`, and `continue` are conventions, not a fixed enum.
- Supported action variants are `primary`, `secondary`, and `danger`.
- Supported dialog types are `info`, `success`, `warning`, `danger`, and `neutral`.
- Dialog requests require `title`.
- Dialog requests may include `content`, `type`, `actions`, and `dismissible`.
- Dialog requests always have props; v1 props are `title`, `content`, `type`, `actions`, `dismissible`, and `source`.
- `type` defaults to `neutral`.
- `dismissible` defaults to `true`.
- `actions` defaults to one `ok` primary action when no actions are supplied.
- `dismissible` controls whether the user can close the dialog without choosing an action.
- Non-dismissible dialogs must include at least one action.
- Dialogs may include multiple actions.
- Action keys must be non-empty.
- Action labels must be non-empty.
- Action keys must be unique inside one dialog.
- `content` is plain text in v1.
- Do not allow raw HTML content in v1.
- Do not add app-defined custom dialog components in v1.
- Non-blocking messages use a separate toast API.
- Dialogs stay blocking.
- Toasts are tracked separately in Roadmap item `#263`.
- Do not model form-like dialogs in v1.
- Form-like dialogs belong to future action or workflow forms.
- Do not add dialog analytics or audit logging in v1.
- Audit the operation, not the dialog.
- Studio keeps a dialog stack.
- New dialogs are pushed on top of the stack.
- Only the top dialog is interactive.
- Closing the top dialog reveals the previous dialog.
- Dialog calls resolve only when their own dialog closes.
- `useDialog()` resolves to the selected action key, or `null` when a dismissible dialog is dismissed.
- Escape only affects the top dismissible dialog.
- Backdrop click does not dismiss dialogs in v1.
- Server-driven dialogs use the same stack as client-created dialogs.
- Do not auto-dedupe dialogs in v1.
- Dialogs must trap focus, support Escape when dismissible, and restore focus after close.
- Destructive actions must use the `danger` action variant.
- Access gate failures can use the shared dialog API for "Access denied", "Session expired", and "Missing setup" states.
- Server responses may include a dialog intent.
- Server errors may include a dialog intent.
- Success responses put `dialog` beside `data`.
- Error responses put `dialog` inside the existing `error` envelope.
- Confirmation tokens live in `error.details.confirmationToken`.
- The server returns dialog data; Studio decides how to render it.
- Server dialog intents cannot include raw HTML.
- Server dialog intents cannot include JavaScript callbacks.
- Server dialog actions return keys only.
- Server confirmation-required flows must use a token and retry model.
- Do not allow server dialog actions to force navigation in v1.

## Shape

```ts
type StudioDialogType = "neutral" | "info" | "success" | "warning" | "danger"
type StudioDialogActionVariant = "primary" | "secondary" | "danger"

type StudioDialogAction = {
  key: string
  label: string
  variant?: StudioDialogActionVariant
}

type StudioDialogRequest = {
  title: string
  content?: string
  type?: StudioDialogType
  actions?: StudioDialogAction[]
  dismissible?: boolean
  source?: "client" | "server"
}

type StudioDialogResult = string | null

type StudioAPIResponse<T> = {
  data?: T
  dialog?: StudioDialogRequest
}

type StudioAPIErrorBody = {
  code: string
  message: string
  details?: Record<string, unknown>
  dialog?: StudioDialogRequest
}

type StudioAPIErrorEnvelope = {
  error: StudioAPIErrorBody
}
```

## SDK Shape

```go
type DialogType string

const (
	DialogNeutral DialogType = "neutral"
	DialogInfo    DialogType = "info"
	DialogSuccess DialogType = "success"
	DialogWarning DialogType = "warning"
	DialogDanger  DialogType = "danger"
)

type DialogActionVariant string

const (
	DialogActionPrimary   DialogActionVariant = "primary"
	DialogActionSecondary DialogActionVariant = "secondary"
	DialogActionDanger    DialogActionVariant = "danger"
)

type DialogAction struct {
	Key     string              `json:"key"`
	Label   string              `json:"label"`
	Variant DialogActionVariant `json:"variant,omitempty"`
}

type Dialog struct {
	Title       string         `json:"title"`
	Content     string         `json:"content,omitempty"`
	Type        DialogType     `json:"type,omitempty"`
	Actions     []DialogAction `json:"actions,omitempty"`
	Dismissible *bool          `json:"dismissible,omitempty"`
}
```
