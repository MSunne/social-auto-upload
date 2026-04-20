import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

// ═══════════════════════════════════════
// Account Store
// ═══════════════════════════════════════

const FALLBACK_PLATFORM_MAP = {
  1: '小红书',
  2: '视频号',
  3: '抖音',
  4: '快手',
}

export const useAccountStore = defineStore('account', () => {
  const accounts = ref([])
  const platforms = ref([])

  const resolvePlatformLabel = (platformType) => {
    const normalizedType = Number(platformType)
    const matched = platforms.value.find(item => Number(item.platformType) === normalizedType)
    return matched?.label || FALLBACK_PLATFORM_MAP[normalizedType] || '未知'
  }

  const refreshAccountPlatformLabels = () => {
    accounts.value = accounts.value.map((item) => ({
      ...item,
      platform: resolvePlatformLabel(item.type),
    }))
  }

  const setPlatforms = (items) => {
    const nextItems = Array.isArray(items) ? items : []
    platforms.value = nextItems
      .filter(item => item && item.visible !== false)
      .map(item => ({
        platformType: Number(item.platformType),
        slug: item.slug,
        label: item.label,
        displayOrder: Number(item.displayOrder || 0),
        visible: item.visible !== false,
        loginEnabled: item.loginEnabled !== false,
        publishEnabled: item.publishEnabled !== false,
        disabledReason: item.disabledReason || '',
      }))
      .sort((a, b) => a.displayOrder - b.displayOrder || a.platformType - b.platformType)
    refreshAccountPlatformLabels()
  }

  /** Parse backend array format → structured objects */
  const setAccounts = (rawData) => {
    accounts.value = rawData.map((item) => ({
      id: item[0],
      type: item[1],
      filePath: item[2],
      name: item[3],
      status: item[4] === -1 ? '验证中' : item[4] === 1 ? '正常' : '异常',
      platform: resolvePlatformLabel(item[1]),
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
    const baseMap = platforms.value.length > 0
      ? platforms.value.reduce((acc, item) => {
          acc[item.platformType] = item.label
          return acc
        }, {})
      : FALLBACK_PLATFORM_MAP
    Object.entries(baseMap).forEach(([, name]) => {
      counts[name] = all.filter((a) => a.platform === name).length
    })
    const active = Object.values(counts).filter((n) => n > 0).length
    return { ...counts, activeCount: active }
  })

  const loginPlatforms = computed(() => platforms.value)

  const publishPlatforms = computed(() => platforms.value)

  const defaultPublishPlatformType = computed(() => {
    const firstEnabled = publishPlatforms.value.find(item => item.publishEnabled)
    return firstEnabled?.platformType || publishPlatforms.value[0]?.platformType || 0
  })

  const getPlatformCapability = (platformType) => {
    const normalizedType = Number(platformType)
    return platforms.value.find(item => Number(item.platformType) === normalizedType) || null
  }

  return {
    accounts,
    platforms,
    stats,
    platformStats,
    loginPlatforms,
    publishPlatforms,
    defaultPublishPlatformType,
    setAccounts,
    setPlatforms,
    addAccount,
    updateAccount,
    deleteAccount,
    getByPlatform,
    getPlatformCapability,
  }
})
