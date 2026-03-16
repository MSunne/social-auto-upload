<template>
  <div class="system-status fade-in">
    <!-- ═══ Device Info ═══ -->
    <div class="section-title"><el-icon><Monitor /></el-icon> 设备信息</div>
    <div class="info-grid">
      <div class="info-card glass-card" v-for="item in deviceInfo" :key="item.label">
        <div class="info-label">{{ item.label }}</div>
        <div class="info-value" :class="item.glow">{{ item.value }}</div>
      </div>
    </div>

    <!-- ═══ Agent Status ═══ -->
    <div class="section-title" style="margin-top: 28px"><el-icon><Connection /></el-icon> 服务状态</div>
    <el-row :gutter="16">
      <el-col :span="12">
        <div class="agent-card glass-card">
          <div class="agent-header">
            <span class="agent-name">CloudAgent</span>
            <span class="agent-dot" :class="agents.cloud ? 'online' : 'offline'"></span>
          </div>
          <div class="agent-status">{{ agents.cloud ? '已连接' : '未连接' }}</div>
        </div>
      </el-col>
      <el-col :span="12">
        <div class="agent-card glass-card">
          <div class="agent-header">
            <span class="agent-name">OmniDrive Agent</span>
            <span class="agent-dot" :class="agents.omnidrive ? 'online' : 'offline'"></span>
          </div>
          <div class="agent-status">{{ agents.omnidrive ? '已连接' : '未连接' }}</div>
        </div>
      </el-col>
    </el-row>

    <!-- ═══ Publish Task Stats ═══ -->
    <div class="section-title" style="margin-top: 28px"><el-icon><DataLine /></el-icon> 任务统计</div>
    <div class="glass-card stats-panel">
      <div class="stats-row">
        <div class="stat-block" v-for="(count, status) in taskStats" :key="status">
          <span class="stat-num">{{ count }}</span>
          <span class="stat-lbl">{{ status }}</span>
        </div>
      </div>
      <el-empty v-if="Object.keys(taskStats).length === 0" description="暂无任务数据" />
    </div>

    <!-- ═══ Material Roots ═══ -->
    <div class="section-title" style="margin-top: 28px"><el-icon><FolderOpened /></el-icon> 素材根目录</div>
    <div class="glass-card roots-panel">
      <div v-if="materialRoots.length > 0">
        <div v-for="(root, i) in materialRoots" :key="i" class="root-item">
          <el-icon><Folder /></el-icon>
          <span>{{ root }}</span>
        </div>
      </div>
      <el-empty v-else description="暂无素材根目录" />
    </div>

    <!-- Refresh -->
    <div style="text-align: center; margin-top: 24px">
      <el-button type="primary" @click="fetchAll" :loading="loading">
        <el-icon><Refresh /></el-icon> 刷新数据
      </el-button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { systemApi } from '@/api/system'
import { publishApi } from '@/api/publish'
import { ElMessage } from 'element-plus'

const loading = ref(false)
const deviceInfo = ref([])
const agents = ref({ cloud: false, omnidrive: false })
const taskStats = ref({})
const materialRoots = ref([])

const fetchAll = async () => {
  loading.value = true
  try {
    const results = await Promise.allSettled([
      systemApi.getSkillStatus(),
      systemApi.getCloudAgentStatus(),
      systemApi.getOmnidriveAgentStatus(),
      publishApi.getPublishTasks(),
    ])

    // Device info
    if (results[0].status === 'fulfilled' && results[0].value?.data) {
      const d = results[0].value.data
      deviceInfo.value = [
        { label: '设备名称', value: d.deviceName || d.device_name || '-' },
        { label: '设备编码', value: d.deviceCode || d.device_code || '-', glow: 'glow-violet' },
        { label: 'MAC 地址', value: d.mac || '-' },
        { label: 'IP 地址', value: d.ip || d.localIp || '-' },
      ]
      materialRoots.value = d.materialRoots || d.material_roots || []
    }

    // Agents
    if (results[1].status === 'fulfilled') {
      agents.value.cloud = results[1].value?.data?.connected ?? false
    }
    if (results[2].status === 'fulfilled') {
      agents.value.omnidrive = results[2].value?.data?.connected ?? false
    }

    // Tasks
    if (results[3].status === 'fulfilled' && results[3].value?.data) {
      const tasks = results[3].value.data
      const byStatus = {}
      tasks.forEach(t => {
        const s = t.status || '未知'
        byStatus[s] = (byStatus[s] || 0) + 1
      })
      taskStats.value = byStatus
    }
  } catch {
    ElMessage.error('获取系统信息失败')
  }
  loading.value = false
}

onMounted(fetchAll)
</script>

<style lang="scss" scoped>
@use '@/styles/variables.scss' as *;

.system-status {
  max-width: 1000px;
}

// ── Info Grid ──
.info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 12px;
}

.info-card {
  padding: 16px 20px;

  .info-label {
    font-size: 12px;
    color: $text-muted;
    margin-bottom: 6px;
  }

  .info-value {
    font-size: 15px;
    font-weight: 600;
    color: $text-primary;
    word-break: break-all;

    &.glow-violet {
      color: $accent-color;
      text-shadow: 0 0 12px $accent-glow;
    }
  }
}

// ── Agent Cards ──
.agent-card {
  padding: 20px;

  .agent-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }

  .agent-name {
    font-size: 15px;
    font-weight: 600;
    color: $text-primary;
  }

  .agent-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    transition: all $transition-base;

    &.online {
      background: $success-color;
      box-shadow: 0 0 12px rgba(0, 255, 136, 0.5);
      animation: pulse-dot 2s ease-in-out infinite;
    }
    &.offline {
      background: $danger-color;
      box-shadow: 0 0 8px rgba(255, 59, 92, 0.3);
    }
  }

  .agent-status {
    font-size: 13px;
    color: $text-secondary;
  }
}

// ── Stats Panel ──
.stats-panel {
  padding: 20px;
}

.stats-row {
  display: flex;
  gap: 24px;
  flex-wrap: wrap;
}

.stat-block {
  display: flex;
  flex-direction: column;
  align-items: center;

  .stat-num {
    font-size: 22px;
    font-weight: 700;
    color: $accent-color;
    text-shadow: 0 0 10px $accent-glow;
  }

  .stat-lbl {
    font-size: 12px;
    color: $text-muted;
    margin-top: 4px;
  }
}

// ── Material Roots ──
.roots-panel {
  padding: 16px 20px;
}

.root-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 0;
  font-size: 13px;
  color: $text-secondary;
  border-bottom: 1px solid $border-color;

  &:last-child { border-bottom: none; }

  .el-icon { color: $cyan-color; }
}
</style>
