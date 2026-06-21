import { useToastStore, type StudioToastRequest } from './toasts.store'

export function useToast() {
  const store = useToastStore()

  return {
    show: (request: StudioToastRequest) => store.show(request),
    success: (title: string, content?: string) => store.show({ title, content, type: 'success' }),
    error: (title: string, content?: string) => store.show({ title, content, type: 'danger' }),
    warning: (title: string, content?: string) => store.show({ title, content, type: 'warning' }),
    info: (title: string, content?: string) => store.show({ title, content, type: 'info' }),
  }
}

export type { StudioToastRequest }
