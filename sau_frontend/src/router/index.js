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
    path: '/task-center',
    name: 'TaskCenter',
    component: () => import('../views/TaskCenter.vue'),
    meta: { title: '任务中心' },
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

router.afterEach((to) => {
  document.title = `${to.meta.title || 'SAU'} — OmniBull`
})

export default router
