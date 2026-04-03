<template>
  <div class="dashboard fade-in">
    <!-- ═══ Stat Cards ═══ -->
    <el-row :gutter="16">
      <el-col :xs="24" :sm="12" :lg="6">
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-icon violet">
              <el-icon :size="22"><User /></el-icon>
            </div>
            <div class="stat-trend" :class="accountStore.stats.abnormal > 0 ? 'warn' : 'good'">
              {{ accountStore.stats.abnormal > 0 ? `${accountStore.stats.abnormal} 异常` : '全部正常' }}
            </div>
          </div>
          <div class="stat-value">{{ accountStore.stats.total }}</div>
          <div class="stat-label">账号总数</div>
          <div class="stat-footer">
            <span>正常 {{ accountStore.stats.normal }}</span>
            <span>异常 {{ accountStore.stats.abnormal }}</span>
          </div>
        </div>
      </el-col>

      <el-col :xs="24" :sm="12" :lg="6">
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-icon cyan">
              <el-icon :size="22"><Platform /></el-icon>
            </div>
            <div class="stat-trend good">已接入</div>
          </div>
          <div class="stat-value">{{ accountStore.platformStats.activeCount }}</div>
          <div class="stat-label">平台数量</div>
          <div class="stat-footer platform-tags">
            <el-tag v-if="accountStore.platformStats['抖音']" size="small" type="danger" effect="dark">抖音 {{ accountStore.platformStats['抖音'] }}</el-tag>
            <el-tag v-if="accountStore.platformStats['快手']" size="small" type="success" effect="dark">快手 {{ accountStore.platformStats['快手'] }}</el-tag>
            <el-tag v-if="accountStore.platformStats['视频号']" size="small" type="warning" effect="dark">视频号 {{ accountStore.platformStats['视频号'] }}</el-tag>
            <el-tag v-if="accountStore.platformStats['小红书']" size="small" effect="dark">小红书 {{ accountStore.platformStats['小红书'] }}</el-tag>
          </div>
        </div>
      </el-col>

      <el-col :xs="24" :sm="12" :lg="6">
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-icon pink">
              <el-icon :size="22"><Document /></el-icon>
            </div>
          </div>
          <div class="stat-value">{{ appStore.materialStats.total }}</div>
          <div class="stat-label">素材总数</div>
          <div class="stat-footer">
            <span>视频 {{ appStore.materialStats.videos }}</span>
            <span>图片 {{ appStore.materialStats.images }}</span>
            <span>其他 {{ appStore.materialStats.others }}</span>
          </div>
        </div>
      </el-col>

      <el-col :xs="24" :sm="12" :lg="6">
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-icon green">
              <el-icon :size="22"><Connection /></el-icon>
            </div>
            <div class="stat-trend" :class="publishStore.taskStats.total > 0 ? 'good' : ''">
              {{ publishStore.taskStats.total }} 条任务
            </div>
          </div>
          <div class="stat-value">{{ publishStore.taskStats.total }}</div>
          <div class="stat-label">发布任务</div>
          <div class="stat-footer">
            <span v-for="(count, status) in publishStore.taskStats.byStatus" :key="status">
              {{ status }} {{ count }}
            </span>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- ═══ Quick Actions ═══ -->
    <div class="section" style="margin-top: 24px">
      <h3 class="section-title">
        <el-icon><Compass /></el-icon> 快捷操作
      </h3>
      <el-row :gutter="16">
        <el-col :xs="12" :sm="12" v-for="action in quickActions" :key="action.path">
          <div class="action-card" @click="router.push(action.path)">
            <div class="action-icon" :class="action.color">
              <el-icon :size="24"><component :is="action.icon" /></el-icon>
            </div>
            <div class="action-title">{{ action.label }}</div>
            <div class="action-desc">{{ action.desc }}</div>
          </div>
        </el-col>
      </el-row>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAccountStore } from '@/stores/account'
import { useAppStore } from '@/stores/app'
import { usePublishStore } from '@/stores/publish'
import { accountApi } from '@/api/account'
import { materialApi } from '@/api/material'
import { publishApi } from '@/api/publish'

const router = useRouter()
const accountStore = useAccountStore()
const appStore = useAppStore()
const publishStore = usePublishStore()
const loading = ref(false)

const quickActions = [
  { path: '/account-management', label: '账号管理', desc: '管理所有平台账号', icon: 'UserFilled', color: 'violet' },
  { path: '/task-center', label: '任务中心', desc: '查看执行任务', icon: 'Tickets', color: 'cyan' },
]

const VIDEO_EXTS = ['.mp4', '.avi', '.mov', '.wmv', '.flv', '.mkv', '.webm']
const IMAGE_EXTS = ['.jpg', '.jpeg', '.png', '.gif', '.bmp', '.webp']

const getFileTypeLabel = (filename) => {
  const lower = filename.toLowerCase()
  if (VIDEO_EXTS.some((e) => lower.endsWith(e))) return '视频'
  if (IMAGE_EXTS.some((e) => lower.endsWith(e))) return '图片'
  return '其他'
}

const getFileTypeColor = (filename) => {
  const label = getFileTypeLabel(filename)
  return { '视频': 'success', '图片': 'warning', '其他': 'info' }[label] || 'info'
}

const fetchData = async () => {
  loading.value = true
  const results = await Promise.allSettled([
    accountApi.getAccounts(),
    materialApi.getAllMaterials(),
    publishApi.getPublishTasks(),
  ])

  if (results[0].status === 'fulfilled' && results[0].value?.data) {
    accountStore.setAccounts(results[0].value.data)
  }
  if (results[1].status === 'fulfilled' && results[1].value?.data) {
    appStore.setMaterials(results[1].value.data)
  }
  if (results[2].status === 'fulfilled' && results[2].value?.data) {
    publishStore.setTasks(results[2].value.data)
  }
  loading.value = false
}

onMounted(fetchData)
</script>

<style lang="scss" scoped>
@use '@/styles/variables.scss' as *;

.dashboard {
  max-width: 1400px;
}

.table-wrapper {
  padding: 4px;
  overflow: hidden;
}

// ── Stat Cards — Neon ──
.stat-card {
  .stat-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
  }

  .stat-icon {
    width: 44px;
    height: 44px;
    border-radius: 12px;
    display: flex;
    align-items: center;
    justify-content: center;

    &.violet {
      background: rgba(177, 73, 255, 0.15);
      box-shadow: 0 0 12px rgba(177, 73, 255, 0.2);
      .el-icon { color: #b149ff; }
    }
    &.cyan {
      background: rgba(0, 245, 212, 0.12);
      box-shadow: 0 0 12px rgba(0, 245, 212, 0.2);
      .el-icon { color: #00f5d4; }
    }
    &.pink {
      background: rgba(255, 45, 120, 0.12);
      box-shadow: 0 0 12px rgba(255, 45, 120, 0.2);
      .el-icon { color: #ff2d78; }
    }
    &.green {
      background: rgba(0, 255, 136, 0.12);
      box-shadow: 0 0 12px rgba(0, 255, 136, 0.2);
      .el-icon { color: #00ff88; }
    }
  }

  .stat-trend {
    font-size: 12px;
    padding: 2px 10px;
    border-radius: 12px;
    background: rgba(255, 255, 255, 0.04);
    color: $text-muted;

    &.good { color: $success-color; background: rgba(0, 255, 136, 0.08); }
    &.warn { color: $danger-color; background: rgba(255, 59, 92, 0.08); }
  }

  .stat-value {
    font-size: 32px;
    font-weight: 700;
    color: $text-primary;
    line-height: 1.1;
  }

  .stat-label {
    font-size: 13px;
    color: $text-secondary;
    margin-top: 4px;
    margin-bottom: 16px;
  }

  .stat-footer {
    border-top: 1px solid $border-color;
    padding-top: 12px;
    display: flex;
    gap: 12px;
    font-size: 12px;
    color: $text-muted;

    &.platform-tags {
      gap: 6px;
      flex-wrap: wrap;
    }
  }
}

// ── Quick Actions — Neon Hover ──
.action-card {
  .action-icon {
    width: 48px;
    height: 48px;
    border-radius: 14px;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: 12px;
    transition: box-shadow $transition-base;

    &.violet {
      background: rgba(177, 73, 255, 0.12);
      .el-icon { color: #b149ff; }
    }
    &.cyan {
      background: rgba(0, 245, 212, 0.10);
      .el-icon { color: #00f5d4; }
    }
    &.pink {
      background: rgba(255, 45, 120, 0.10);
      .el-icon { color: #ff2d78; }
    }
    &.green {
      background: rgba(0, 255, 136, 0.10);
      .el-icon { color: #00ff88; }
    }
  }

  &:hover .action-icon {
    &.violet { box-shadow: 0 0 20px rgba(177, 73, 255, 0.3); }
    &.cyan   { box-shadow: 0 0 20px rgba(0, 245, 212, 0.3); }
    &.pink   { box-shadow: 0 0 20px rgba(255, 45, 120, 0.3); }
    &.green  { box-shadow: 0 0 20px rgba(0, 255, 136, 0.3); }
  }

  .action-title {
    font-size: 15px;
    font-weight: 600;
    color: $text-primary;
    margin-bottom: 4px;
  }

  .action-desc {
    font-size: 12px;
    color: $text-muted;
  }
}

// ── Sections ──
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0;
}
</style>
