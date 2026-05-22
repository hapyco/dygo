import { createRouter, createWebHistory, type NavigationGuardWithThis, type RouteLocationRaw } from 'vue-router'

import LoginPage from '@/features/auth/LoginPage.vue'
import HomePage from '@/pages/HomePage.vue'
import NotFoundPage from '@/pages/NotFoundPage.vue'
import RecordFormPage from '@/pages/RecordFormPage.vue'
import RecordsPage from '@/pages/RecordsPage.vue'
import { useAuthStore } from '@/stores/auth.store'
import { pinia } from '@/stores/pinia'
import { isRootReservedSlug, routeParam, RouteName } from './routes'

declare module 'vue-router' {
  interface RouteMeta {
    public?: boolean
    redirectIfAuthenticated?: boolean
  }
}

const rejectReservedEntity: NavigationGuardWithThis<undefined> = (to): RouteLocationRaw | undefined => {
  const entity = routeParam(to.params.entity as string | string[])
  if (!isRootReservedSlug(entity)) {
    return undefined
  }

  return {
    name: RouteName.NotFound,
    params: { pathMatch: [entity] },
    replace: true,
  }
}

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: RouteName.Login,
      component: LoginPage,
      meta: { public: true, redirectIfAuthenticated: true },
    },
    {
      path: '/',
      name: RouteName.Home,
      component: HomePage,
    },
    {
      path: '/:entity',
      name: RouteName.EntityRecords,
      component: RecordsPage,
      beforeEnter: rejectReservedEntity,
      props: (route) => ({ entity: routeParam(route.params.entity as string | string[]) }),
    },
    {
      path: '/:entity/new',
      name: RouteName.RecordNew,
      component: RecordFormPage,
      beforeEnter: rejectReservedEntity,
      props: (route) => ({
        entity: routeParam(route.params.entity as string | string[]),
        mode: 'new',
      }),
    },
    {
      path: '/:entity/:recordName',
      name: RouteName.RecordDetail,
      component: RecordFormPage,
      beforeEnter: rejectReservedEntity,
      props: (route) => ({
        entity: routeParam(route.params.entity as string | string[]),
        recordName: routeParam(route.params.recordName as string | string[]),
        mode: 'record',
      }),
    },
    {
      path: '/:pathMatch(.*)*',
      name: RouteName.NotFound,
      component: NotFoundPage,
      meta: { public: true },
    },
  ],
})

router.beforeEach(async (to): Promise<RouteLocationRaw | undefined> => {
  const authStore = useAuthStore(pinia)
  const user = await authStore.loadCurrentUser()

  if (to.meta.redirectIfAuthenticated && user) {
    return { name: RouteName.Home }
  }

  if (to.meta.public || user) {
    return undefined
  }

  return {
    name: RouteName.Login,
    query: { redirect: to.fullPath },
  }
})
