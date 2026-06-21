# Toasts

Status: v1 decision log.

Studio needs a small non-blocking message surface for short-lived feedback.

## Decided

- Use `toast` as the public Studio term.
- Use `useToast()` as the frontend API name.
- Toasts are transient, non-blocking messages.
- Dialogs stay blocking; toasts do not replace dialogs.
- Persistent system items are future notifications, not v1 toasts.
- Toasts render in the bottom-right corner.
- Toasts stack newest-first.
- Newest toast appears closest to the bottom-right edge; older toasts move upward.
- Toasts auto-dismiss by default.
- Toasts can be manually dismissed.
- Manual dismissal means the user closes the toast; v1 does not expose programmatic close handles.
- V1 toast types are `info`, `success`, `warning`, and `danger`.
- V1 toast requests require `title`.
- V1 toast requests may include `content`, `type`, and `duration`.
- Empty `title` is invalid.
- Empty `content` is allowed.
- `type` defaults to `info`.
- `duration` is milliseconds.
- Missing `duration` uses the Studio default duration.
- Default duration is `4000` ms.
- `duration: 0` means sticky until dismissed.
- Negative `duration` is invalid.
- V1 toasts do not include actions.
- If a message needs required user action, use a dialog.
- `useToast()` exposes `show`, `success`, `error`, `warning`, and `info`.
- `show` accepts the full toast request shape.
- `success`, `error`, `warning`, and `info` are convenience methods for common toast types.
- `error` creates a `danger` toast.
- Server responses may include a single toast intent in v1.
- Server `toast` is best-effort and must not change API request semantics.
- Use `dygo.Toast` as the public SDK type name.
- Server response envelopes should use a single `toast` field in v1.
- Success responses put `toast` beside `data`.
- Error responses put `toast` inside the existing `error` envelope.
- Do not add `toasts` arrays in v1.
- Duplicate identical toasts are allowed in v1.
- Toasts are not persisted.
- Toasts are not written to Activity.

## Shape

```ts
type StudioToastType = "info" | "success" | "warning" | "danger"

type StudioToastRequest = {
  title: string
  content?: string
  type?: StudioToastType
  duration?: number
}

type UseToast = {
  show(request: StudioToastRequest): void
  success(title: string, content?: string): void
  error(title: string, content?: string): void
  warning(title: string, content?: string): void
  info(title: string, content?: string): void
}
```

## SDK Shape

```go
type ToastType string

const (
	ToastInfo    ToastType = "info"
	ToastSuccess ToastType = "success"
	ToastWarning ToastType = "warning"
	ToastDanger  ToastType = "danger"
)

type Toast struct {
	Title    string    `json:"title"`
	Content  string    `json:"content,omitempty"`
	Type     ToastType `json:"type,omitempty"`
	Duration int       `json:"duration,omitempty"`
}
```
