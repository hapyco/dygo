import { getCurrentInstance, onActivated, onDeactivated, ref, type Ref } from 'vue'

export function useSheetActive(): Ref<boolean> {
  const active = ref(true)
  if (getCurrentInstance()) {
    onActivated(() => { active.value = true })
    onDeactivated(() => { active.value = false })
  }
  return active
}
