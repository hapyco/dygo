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
- `dismissible` controls whether the user can close the dialog without choosing an action.
- Non-dismissible dialogs must include at least one action.
- Dialogs may include multiple actions.
- `content` is plain text in v1.
- Do not allow raw HTML content in v1.
- Do not add app-defined custom dialog components in v1.
- Studio keeps a dialog stack.
- New dialogs are pushed on top of the stack.
- Only the top dialog is interactive.
- Closing the top dialog reveals the previous dialog.
- Dialog calls resolve only when their own dialog closes.
- Escape only affects the top dismissible dialog.
- Server-driven dialogs use the same stack as client-created dialogs.
- Do not auto-dedupe dialogs in v1.
- Dialogs must trap focus, support Escape when dismissible, and restore focus after close.
- Destructive actions must use the `danger` action variant.
- Access gate failures can use the shared dialog API for "Access denied", "Session expired", and "Missing setup" states.
- Server responses may include a dialog intent.
- Server errors may include a dialog intent.
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

type StudioAPIResponse<T> = {
  data?: T
  dialog?: StudioDialogRequest
}

type StudioAPIError = {
  code: string
  message: string
  dialog?: StudioDialogRequest
}

type StudioConfirmationRequired = {
  code: "confirmation_required"
  confirmationToken: string
  dialog: StudioDialogRequest
}
```

## Pending

- Decide exact frontend API name.
- Decide whether non-blocking notifications need a separate `toast` API.
- Decide how form-like dialogs should be modeled later.
- Decide how dialog analytics or audit logging should work for sensitive actions.
