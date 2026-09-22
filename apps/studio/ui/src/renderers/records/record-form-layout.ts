import type { MetadataFormLayout, MetadataFormItem } from '@/features/metadata/metadata.api'

export function resolveFormLayout(
  authored: MetadataFormLayout | null | undefined,
  fields: { name: string }[],
  label: string,
): { form: MetadataFormLayout; showTabStrip: boolean } {
  const fieldItems = (names: { name: string }[]): MetadataFormItem[] => names.map(({ name }) => ({ kind: 'field', name }))
  if (!authored?.tabs.length) {
    return {
      form: { tabs: [{ key: 'default', label, items: fieldItems(fields) }] },
      showTabStrip: false,
    }
  }

  const assigned = new Set(authored.tabs.flatMap((tab) => tab.items.filter((item) => item.kind === 'field').map((item) => item.name)))
  // Runtime fields include the manual Record ID, which is not an authored storage Field.
  const extraFields = fieldItems(fields.filter((field) => !assigned.has(field.name)))
  return {
    form: {
      tabs: authored.tabs.map((tab, index) => ({
        ...tab,
        items: index === 0 ? [...extraFields, ...tab.items] : tab.items,
      })),
    },
    showTabStrip: true,
  }
}
