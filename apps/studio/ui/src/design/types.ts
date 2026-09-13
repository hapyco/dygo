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
  sortable?: boolean
  formatValue?: (value: unknown) => string
}

export type DataTableRow = Record<string, unknown>

export type DataTableRowKey = string | number

export type DataTableSortDirection = 'asc' | 'desc'

export type DataTableSort = {
  key: string
  direction: DataTableSortDirection
}

export type DataTableState = 'ready' | 'loading' | 'empty' | 'forbidden' | 'unauthenticated' | 'error'

export type SegmentedControlValue = string | number

export type SegmentedControlOption = {
  value: SegmentedControlValue
  label: string
  disabled?: boolean
}
