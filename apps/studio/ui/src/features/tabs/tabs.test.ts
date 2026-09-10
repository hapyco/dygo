import assert from 'node:assert/strict'
import test from 'node:test'

import { RouteName } from '../../router/routes.ts'
import {
  closeStudioTab,
  cycleStudioTab,
  morphStudioTab,
  normalizeStudioTabs,
  shouldMorphNewRecordTab,
  tabCacheKey,
  tabLabelForRoute,
  upsertStudioTab,
} from './tabs.ts'

const home = { path: '/', fullPath: '/', label: 'Home' }
const customers = { path: '/customers', fullPath: '/customers?status=open', label: 'Customers' }
const draft = { path: '/customers/new', fullPath: '/customers/new', label: 'New Customer' }
const saved = { path: '/customers/CUS-1', fullPath: '/customers/CUS-1', label: 'Customer / CUS-1' }

test('normalizes open tabs and drops duplicate paths', () => {
  const tabs = normalizeStudioTabs([
    customers,
    { path: '/customers', fullPath: '/customers', label: 'Old' },
    { path: 'customers', fullPath: '/customers', label: 'Bad' },
    home,
  ])
  assert.equal(tabs.length, 2)
  assert.equal(tabs[0]?.label, 'Customers')
  assert.equal(tabs[0]?.fullPath, '/customers?status=open')
})

test('upsert updates query state without changing tab order', () => {
  const tabs = upsertStudioTab([home, customers], { ...customers, fullPath: '/customers?q=acme', label: 'Customers' })
  assert.deepEqual(tabs.map(tab => tab.path), ['/', '/customers'])
  assert.equal(tabs[1]?.fullPath, '/customers?q=acme')
})

test('creating a Record replaces the New Record tab in place', () => {
  assert.equal(shouldMorphNewRecordTab(draft.path, saved.path), true)
  assert.equal(shouldMorphNewRecordTab(customers.path, saved.path), false)
  const tabs = morphStudioTab([home, draft, customers], draft.path, saved)
  assert.deepEqual(tabs.map(tab => tab.path), ['/', '/customers/CUS-1', '/customers'])
})

test('closing the active tab activates a neighbor and keeps the last tab', () => {
  const closed = closeStudioTab([home, customers, saved], customers.path, customers.path)
  assert.deepEqual(closed.tabs.map(tab => tab.path), ['/', '/customers/CUS-1'])
  assert.equal(closed.activate?.path, '/customers/CUS-1')
  const last = closeStudioTab([home], home.path, home.path)
  assert.equal(last.closed, false)
  assert.equal(last.tabs.length, 1)
  assert.equal(last.activate, null)
})

test('cycle wraps around open tabs', () => {
  const tabs = [home, customers, saved]
  assert.equal(cycleStudioTab(tabs, customers.path, 1)?.path, saved.path)
  assert.equal(cycleStudioTab(tabs, home.path, -1)?.path, saved.path)
})

test('tab labels follow Entity, Record, and Page routes', () => {
  const label = (slug: string) => slug === 'customers' ? 'Customers' : ''
  assert.equal(tabLabelForRoute({ name: RouteName.Home, path: '/', fullPath: '/', params: {} }, label), 'Home')
  assert.equal(tabLabelForRoute({ name: RouteName.RecordNew, path: '/customers/new', fullPath: '/customers/new', params: { entity: 'customers' } }, label), 'New Customers')
  assert.equal(tabLabelForRoute({ name: RouteName.RecordDetail, path: '/customers/CUS-1', fullPath: '/customers/CUS-1', params: { entity: 'customers', recordName: 'CUS-1' } }, label), 'Customers / CUS-1')
  assert.equal(tabLabelForRoute({ name: 'page:crm:pipeline', path: '/pipeline', fullPath: '/pipeline', params: {} }, label), 'Pipeline')
})

test('each newly opened tab receives a distinct cache identity', () => {
  const first = normalizeStudioTabs([{ path: '/customers/CUS-1', fullPath: '/customers/CUS-1', label: 'Customer' }])
  const second = normalizeStudioTabs([{ path: '/customers/CUS-1', fullPath: '/customers/CUS-1', label: 'Customer' }])
  assert.notEqual(first[0]?.cacheKey, second[0]?.cacheKey)
})
