export type ControlSize = 'xs' | 'sm' | 'md'

export type FieldOption = {
  value: string
  label: string
  disabled?: boolean
}

export type BadgeVariant = 'neutral' | 'accent' | 'success' | 'warning' | 'danger' | 'info'

export type TextInputType = 'text' | 'email' | 'password' | 'number' | 'date' | 'search'

export type DataTableColumn = {
  key: string
  label: string
}

export type DataTableRow = Record<string, unknown>

export type DataTableRowKey = string | number

export type SegmentedControlValue = string | number

export type SegmentedControlOption = {
  value: SegmentedControlValue
  label: string
  disabled?: boolean
}
