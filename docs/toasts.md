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
- Toasts auto-dismiss by default.
- Toasts can be manually dismissed.
- V1 toast types are `info`, `success`, `warning`, and `danger`.
- V1 toast requests require `title`.
- V1 toast requests may include `content`, `type`, and `duration`.
- V1 toasts do not include actions.
- If a message needs required user action, use a dialog.
- Server responses may include toast intents later, but v1 can start with frontend calls.

## Shape

```ts
type StudioToastType = "info" | "success" | "warning" | "danger"

type StudioToastRequest = {
  title: string
  content?: string
  type?: StudioToastType
  duration?: number
}
```
