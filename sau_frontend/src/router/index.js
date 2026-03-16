import { createRouter, createWebHashHistory } from 'vue-router'

const routes = [
  {
    path: '/',
    name: 'Dashboard',
    component: () => import('../views/Dashboard.vue'),
    meta: { title: '仪表盘' },
  },
  {
    path: '/account-management',
    name: 'AccountManagement',
    component: () => import('../views/AccountManagement.vue'),
    meta: { title: '账号管理' },
  },
  {
    path: '/material-management',
    name: 'MaterialManagement',
    component: () => import('../views/MaterialManagement.vue'),
    meta: { title: '素材管理' },
  },
  {
    path: '/publish-center',
    name: 'PublishCenter',
    component: () => import('../views/PublishCenter.vue'),
    meta: { title: '发布中心' },
  },
  {
    path: '/system-status',
    name: 'SystemStatus',
    component: () => import('../views/SystemStatus.vue'),
    meta: { title: '系统状态' },
  },
  {
    path: '/about',
    name: 'About',
    component: () => import('../views/About.vue'),
    meta: { title: '关于' },
  },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

router.afterEach((to) => {
  document.title = `${to.meta.title || 'SAU'} — OmniBull`
})

export default router