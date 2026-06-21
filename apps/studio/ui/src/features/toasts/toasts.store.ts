import { defineStore } from 'pinia'

const defaultDuration = 4000

export type StudioToastType = 'info' | 'success' | 'warning' | 'danger'

export type StudioToastRequest = {
  title: string
  content?: string
  type?: StudioToastType
  duration?: number
}

export type StudioToast = Required<StudioToastRequest> & {
  id: number
}

let nextToastID = 1
const toastTimers = new Map<number, ReturnType<typeof setTimeout>>()

export const useToastStore = defineStore('toasts', {
  state: () => ({
    toasts: [] as StudioToast[],
  }),

  actions: {
    show(request: StudioToastRequest) {
      const toast = normalizeToast(request)
      this.toasts.push(toast)
      if (toast.duration > 0) {
        toastTimers.set(toast.id, setTimeout(() => this.dismiss(toast.id), toast.duration))
      }
    },

    dismiss(id: number) {
      const timer = toastTimers.get(id)
      if (timer) {
        clearTimeout(timer)
        toastTimers.delete(id)
      }
      this.toasts = this.toasts.filter((toast) => toast.id !== id)
    },
  },
})

function normalizeToast(request: StudioToastRequest): StudioToast {
  const title = request.title.trim()
  if (!title) {
    throw new Error('toast title is required')
  }
  if (request.duration !== undefined && request.duration < 0) {
    throw new Error('toast duration must not be negative')
  }

  return {
    id: nextToastID++,
    title,
    content: request.content?.trim() ?? '',
    type: request.type ?? 'info',
    duration: request.duration ?? defaultDuration,
  }
}
