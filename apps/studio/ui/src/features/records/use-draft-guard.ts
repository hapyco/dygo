import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute } from 'vue-router'
import { useDialog } from '@/features/dialogs/use-dialog'
import { useNavigationStore } from '@/stores/navigation.store'
import { draftProtection } from './draft-protection'

export function useDraftGuard(dirty: () => boolean, saving: () => boolean) {
  const dialog = useDialog()
  const route = useRoute()
  const navigation = useNavigationStore()
  const sheetPath = route.path
  const confirmDiscard = draftProtection(dirty, saving, () => dialog.confirm({
      title: 'Discard changes?', content: 'Unsaved changes will be lost.', type: 'warning',
      actions: [{ key: 'cancel', label: 'Keep editing', variant: 'secondary' }, { key: 'discard', label: 'Discard changes', variant: 'danger' }],
    }).then(result => result === 'discard'))
  onBeforeRouteLeave(() => navigation.openTabs.some(tab => tab.path === sheetPath) ? true : confirmDiscard())
  onBeforeRouteUpdate((to, from) => to.path === from.path || navigation.openTabs.some(tab => tab.path === sheetPath) ? true : confirmDiscard())
  return confirmDiscard
}
