import { defineStore } from 'pinia'

export type StudioDialogType = 'neutral' | 'info' | 'success' | 'warning' | 'danger'
export type StudioDialogActionVariant = 'primary' | 'secondary' | 'danger'
export type StudioDialogSource = 'client' | 'server'
export type StudioDialogResult = string | null

export type StudioDialogAction = {
  key: string
  label: string
  variant?: StudioDialogActionVariant
}

export type StudioDialogRequest = {
  title: string
  content?: string
  type?: StudioDialogType
  actions?: StudioDialogAction[]
  dismissible?: boolean
  source?: StudioDialogSource
}

export type StudioDialog = {
  id: number
  title: string
  content: string
  type: StudioDialogType
  actions: Required<StudioDialogAction>[]
  dismissible: boolean
  source: StudioDialogSource
}

let nextDialogID = 1
const resolvers = new Map<number, (result: StudioDialogResult) => void>()

export const useDialogStore = defineStore('dialogs', {
  state: () => ({
    stack: [] as StudioDialog[],
  }),

  getters: {
    topDialog: (state): StudioDialog | null => state.stack.at(-1) ?? null,
  },

  actions: {
    open(request: StudioDialogRequest): Promise<StudioDialogResult> {
      const dialog = normalizeDialog(request)
      this.stack.push(dialog)

      return new Promise((resolve) => {
        resolvers.set(dialog.id, resolve)
      })
    },

    selectAction(id: number, key: string) {
      const dialog = this.topDialog
      if (!dialog || dialog.id !== id || !dialog.actions.some((action) => action.key === key)) {
        return
      }

      this.closeTop(key)
    },

    dismissTop() {
      if (!this.topDialog?.dismissible) {
        return
      }

      this.closeTop(null)
    },

    closeTop(result: StudioDialogResult) {
      const dialog = this.topDialog
      if (!dialog) {
        return
      }

      this.stack.pop()
      resolvers.get(dialog.id)?.(result)
      resolvers.delete(dialog.id)
    },
  },
})

function normalizeDialog(request: StudioDialogRequest): StudioDialog {
  const title = request.title.trim()
  if (!title) {
    throw new Error('dialog title is required')
  }

  const dismissible = request.dismissible ?? true
  const actions = normalizeActions(request.actions)
  if (actions.length === 0) {
    if (!dismissible) {
      throw new Error('non-dismissible dialog requires an action')
    }
    actions.push({ key: 'ok', label: 'OK', variant: 'primary' })
  }

  return {
    id: nextDialogID++,
    title,
    content: request.content?.trim() ?? '',
    type: request.type ?? 'neutral',
    actions,
    dismissible,
    source: request.source ?? 'client',
  }
}

function normalizeActions(actions: StudioDialogAction[] | undefined): Required<StudioDialogAction>[] {
  const normalized: Required<StudioDialogAction>[] = []
  const keys = new Set<string>()

  for (const action of actions ?? []) {
    const key = action.key.trim()
    const label = action.label.trim()
    if (!key) {
      throw new Error('dialog action key is required')
    }
    if (!label) {
      throw new Error('dialog action label is required')
    }
    if (keys.has(key)) {
      throw new Error(`duplicate dialog action key: ${key}`)
    }
    keys.add(key)
    normalized.push({ key, label, variant: action.variant ?? 'secondary' })
  }

  return normalized
}
