import type { Component } from 'vue'

export type PageHeaderAction = {
  label: string
  icon?: Component
  variant?: 'primary' | 'secondary' | 'ghost'
  disabled?: boolean
  loading?: boolean
  onSelect?: () => void
}

export type ShellNavItem = {
  label: string
  to: string
  icon?: Component
  current?: boolean
}
