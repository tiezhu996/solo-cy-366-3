import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { USER_ROLE } from '@/constants'
import { resolveRouteGuard } from './guard'

declare module 'vue-router' {
  interface RouteMeta {
    title?: string
    roles?: string[]
  }
}

const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'Login', component: () => import('@/pages/Login.vue') },
  {
    path: '/',
    component: () => import('@/layouts/MainLayout.vue'),
    redirect: '/dashboard',
    children: [
      { path: 'dashboard', name: 'Dashboard', component: () => import('@/pages/Dashboard.vue') },
      { path: 'stations', name: 'Stations', component: () => import('@/pages/Stations.vue') },
      { path: 'recharge', name: 'Recharge', component: () => import('@/pages/Recharge.vue') },
      { path: 'reservations', name: 'Reservations', component: () => import('@/pages/Reservations.vue') },
      { path: 'sessions', name: 'Sessions', component: () => import('@/pages/Sessions.vue') },
      { path: 'tournaments', name: 'Tournaments', component: () => import('@/pages/Tournaments.vue') },
      { path: 'audits', name: 'Audits', component: () => import('@/pages/Audits.vue'), meta: { roles: [USER_ROLE.ADMIN, USER_ROLE.STAFF] } },
      { path: 'boot-code', name: 'BootCode', component: () => import('@/pages/BootCode.vue'), meta: { title: '我的开机码', roles: [USER_ROLE.MEMBER] } },
      { path: 'scan-boot', name: 'ScanBoot', component: () => import('@/pages/ScanBoot.vue'), meta: { title: '扫码开机', roles: [USER_ROLE.ADMIN, USER_ROLE.STAFF] } },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
  const authStore = useAuthStore()
  const result = resolveRouteGuard({
    token: authStore.token,
    role: authStore.user?.role,
    name: to.name,
    roles: to.meta.roles as string[] | undefined,
  })
  return result === true ? true : result
})

export default router
