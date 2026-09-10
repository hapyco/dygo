import { useRouter } from 'vue-router'

import { useDialog } from '@/features/dialogs/use-dialog'
import { useNavigationStore } from '@/stores/navigation.store'

export function useCloseTab() {
  const dialog = useDialog()
  const navigation = useNavigationStore()
  const router = useRouter()

  return async (path: string) => {
    if (navigation.isTabDirty(path)) {
      const result = await dialog.confirm({
        title: 'Discard changes?',
        content: 'Unsaved changes will be lost.',
        type: 'warning',
        actions: [
          { key: 'cancel', label: 'Keep editing', variant: 'secondary' },
          { key: 'discard', label: 'Discard changes', variant: 'danger' },
        ],
      })
      if (result !== 'discard') return
      navigation.setTabDirty(path, false)
    }

    const activate = navigation.closeTab(path, router.currentRoute.value.path)
    if (activate) await router.push(activate.fullPath)
  }
}
