import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

// ═══════════════════════════════════════
// Account Store
// ═══════════════════════════════════════

const PLATFORM_MAP = {
  1: '小红书',
  2: '视频号',
  3: '抖音',
  4: '快手',
}

export const useAccountStore = defineStore('account', () => {
  const accounts = ref([])

  /** Parse backend array format → structured objects */
  const setAccounts = (rawData) => {
    accounts.value = rawData.map((item) => ({
      id: item[0],
      type: item[1],
      filePath: item[2],
      name: item[3],
      status: item[4] === -1 ? '验证中' : item[4] === 1 ? '正常' : '异常',
      platform: PLATFORM_MAP[item[1]] || '未知',
    }))
  }

  const addAccount = (account) => {
    accounts.value.push(account)
  }

  const updateAccount = (id, updated) => {
    const idx = accounts.value.findIndex((a) => a.id === id)
    if (idx !== -1) {
      accounts.value[idx] = { ...accounts.value[idx], ...updated }
    }
  }

  const deleteAccount = (id) => {
    accounts.value = accounts.value.filter((a) => a.id !== id)
  }

  const getByPlatform = (platform) => {
    return accounts.value.filter((a) => a.platform === platform)
  }

  // ── Computed stats ──
  const stats = computed(() => {
    const all = accounts.value
    const normal = all.filter((a) => a.status === '正常').length
    const abnormal = all.filter((a) => a.status !== '正常' && a.status !== '验证中').length
    return { total: all.length, normal, abnormal }
  })

  const platformStats = computed(() => {
    const all = accounts.value
    const counts = {}
    Object.entries(PLATFORM_MAP).forEach(([, name]) => {
      counts[name] = all.filter((a) => a.platform === name).length
    })
    const active = Object.values(counts).filter((n) => n > 0).length
    return { ...counts, activeCount: active }
  })

  return {
    accounts,
    stats,
    platformStats,
    setAccounts,
    addAccount,
    updateAccount,
    deleteAccount,
    getByPlatform,
  }
})