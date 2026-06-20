import { useDialogStore, type StudioDialogRequest, type StudioDialogResult } from './dialogs.store'

export function useDialog() {
  const store = useDialogStore()

  return {
    open: (request: StudioDialogRequest): Promise<StudioDialogResult> => store.open(request),
    alert: (request: StudioDialogRequest): Promise<StudioDialogResult> => store.open({
      ...request,
      actions: request.actions ?? [{ key: 'ok', label: 'OK', variant: 'primary' }],
    }),
    confirm: (request: StudioDialogRequest): Promise<StudioDialogResult> => store.open({
      ...request,
      dismissible: request.dismissible ?? false,
      actions: request.actions ?? [
        { key: 'cancel', label: 'Cancel', variant: 'secondary' },
        { key: 'confirm', label: 'Confirm', variant: 'primary' },
      ],
    }),
    choose: (request: StudioDialogRequest): Promise<StudioDialogResult> => store.open(request),
  }
}

export type { StudioDialogRequest, StudioDialogResult }
