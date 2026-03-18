<template>
  <div class="account-management fade-in">
    <!-- ═══ Stats Bar ═══ -->
    <el-row :gutter="12" class="stats-bar">
      <el-col :span="6">
        <div class="mini-stat glass-card">
          <span class="val">{{ accountStore.stats.total }}</span>
          <span class="lbl">总数</span>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="mini-stat glass-card good">
          <span class="val">{{ accountStore.stats.normal }}</span>
          <span class="lbl">正常</span>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="mini-stat glass-card warn">
          <span class="val">{{ accountStore.stats.abnormal }}</span>
          <span class="lbl">异常</span>
        </div>
      </el-col>
      <el-col :span="6">
        <div class="mini-stat glass-card">
          <span class="val">{{ accountStore.platformStats.activeCount }}</span>
          <span class="lbl">平台</span>
        </div>
      </el-col>
    </el-row>

    <!-- ═══ Toolbar ═══ -->
    <div class="toolbar glass-card">
      <el-input
        v-model="searchQuery"
        prefix-icon="Search"
        placeholder="搜索账号…"
        clearable
        class="search-input"
      />
      <div class="toolbar-actions">
        <el-button type="primary" @click="showAddDialog = true">
          <el-icon><Plus /></el-icon> 添加账号
        </el-button>
        <el-button @click="batchValidate" :loading="validating">
          <el-icon><CircleCheck /></el-icon> 批量验证
        </el-button>
        <el-button @click="fetchAccounts">
          <el-icon><Refresh /></el-icon>
        </el-button>
      </div>
    </div>

    <!-- ═══ Account Table ═══ -->
    <div class="glass-card table-wrapper">
      <el-table :data="filteredAccounts" v-loading="loading" style="width: 100%">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="platform" label="平台" width="100">
          <template #default="{ row }">
            <el-tag size="small" effect="dark" :type="platformTagType(row.platform)">{{ row.platform }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="账号名" min-width="160" />
        <el-table-column label="Cookie" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === '正常' ? 'success' : 'danger'" size="small" effect="dark">
              {{ row.status === '正常' ? '有效' : '无效' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="primary" plain @click="validateOne(row.id)" :loading="row._validating">验证</el-button>
            <el-button size="small" plain @click="exportCookie(row)">
              <el-icon><Download /></el-icon> Cookie
            </el-button>
            <el-button size="small" type="danger" plain @click="deleteAccount(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- ═══ Add Account Dialog ═══ -->
    <el-dialog v-model="showAddDialog" title="添加账号" width="480px" destroy-on-close>
      <el-form label-width="80px">
        <el-form-item label="平台">
          <el-select v-model="newAccount.platform" placeholder="选择平台" style="width: 100%">
            <el-option label="抖音" value="douyin" />
            <el-option label="快手" value="kuaishou" />
            <el-option label="视频号" value="shipinhao" />
            <el-option label="小红书" value="xiaohongshu" />
          </el-select>
        </el-form-item>
        <el-form-item label="账号名">
          <el-input v-model="newAccount.name" placeholder="自定义账号名称" />
        </el-form-item>
      </el-form>

      <!-- Login Progress -->
      <div v-if="loginState.started" class="login-progress">
        <div class="login-log glass-card">
          <p v-for="(msg, i) in loginState.messages" :key="i" :class="msg.type">{{ msg.text }}</p>
        </div>
      </div>

      <template #footer>
        <el-button @click="showAddDialog = false">取消</el-button>
        <el-button type="primary" @click="startLogin" :loading="loginState.started" :disabled="!newAccount.platform || !newAccount.name">
          {{ loginState.started ? '登录中…' : '扫码登录' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- ═══ Cookie Import Dialog ═══ -->
    <el-dialog v-model="showImportDialog" title="导入 Cookie" width="500px">
      <el-upload drag :auto-upload="true" :action="uploadCookieUrl" :headers="authHeaders" :on-success="onCookieUploaded" accept=".json,.txt">
        <el-icon class="el-icon--upload"><Upload /></el-icon>
        <div class="el-upload__text">拖拽 Cookie 文件到这里，或<em>点击上传</em></div>
      </el-upload>
      <template #footer>
        <el-button @click="showImportDialog = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useAccountStore } from '@/stores/account'
import { accountApi } from '@/api/account'
import { createSSE } from '@/utils/request'
import { ElMessage, ElMessageBox } from 'element-plus'

const accountStore = useAccountStore()
const loading = ref(false)
const validating = ref(false)
const searchQuery = ref('')
const showAddDialog = ref(false)
const showImportDialog = ref(false)

const newAccount = ref({ platform: '', name: '' })
const loginState = ref({ started: false, messages: [] })

const apiBase = import.meta.env.VITE_API_BASE_URL || 'http://localhost:5409'
const uploadCookieUrl = `${apiBase}/uploadCookie`
const authHeaders = computed(() => ({ Authorization: `Bearer ${localStorage.getItem('token') || ''}` }))

const filteredAccounts = computed(() => {
  if (!searchQuery.value) return accountStore.accounts
  const q = searchQuery.value.toLowerCase()
  return accountStore.accounts.filter(a => a.name?.toLowerCase().includes(q) || a.platform?.toLowerCase().includes(q))
})

const platformTagType = (p) => ({ '抖音': 'danger', '快手': 'success', '视频号': 'warning', '小红书': '' }[p] || 'info')

const fetchAccounts = async () => {
  loading.value = true
  try {
    const res = await accountApi.getAccounts()
    if (res?.data) accountStore.setAccounts(res.data)
  } catch { ElMessage.error('获取账号失败') }
  loading.value = false
}

const validateOne = async (id) => {
  const acc = accountStore.accounts.find(a => a.id === id)
  if (acc) acc._validating = true
  try {
    const res = await accountApi.validateAccount(id)
    const row = Array.isArray(res?.data) ? res.data : null
    const isValid = row?.[4] === 1
    ElMessage[isValid ? 'success' : 'warning'](isValid ? '验证成功' : '账号状态异常，需要重新登录')
    fetchAccounts()
  } catch { ElMessage.error('验证失败') }
  if (acc) acc._validating = false
}

const batchValidate = async () => {
  validating.value = true
  try {
    await accountApi.getValidAccounts()
    ElMessage.success('批量验证完成')
    fetchAccounts()
  } catch { ElMessage.error('批量验证失败') }
  validating.value = false
}

const deleteAccount = async (id) => {
  await ElMessageBox.confirm('确定删除该账号？', '确认', { type: 'warning' })
  try {
    await accountApi.deleteAccount(id)
    ElMessage.success('已删除')
    fetchAccounts()
  } catch { ElMessage.error('删除失败') }
}

const exportCookie = async (row) => {
  try {
    const url = `${apiBase}/downloadCookie?id=${row.id}`
    window.open(url, '_blank')
  } catch { ElMessage.error('导出失败') }
}

const startLogin = () => {
  loginState.value = { started: true, messages: [{ text: '正在初始化登录…', type: 'info' }] }
  const sseUrl = accountApi.getLoginSSEUrl(newAccount.value.platform, newAccount.value.name)
  const es = createSSE(sseUrl)

  es.onmessage = (e) => {
    loginState.value.messages.push({ text: e.data, type: 'info' })
  }
  es.addEventListener('qr', (e) => {
    loginState.value.messages.push({ text: `二维码已生成，请扫码`, type: 'success' })
  })
  es.addEventListener('done', () => {
    loginState.value.messages.push({ text: '✅ 登录成功！', type: 'success' })
    es.close()
    loginState.value.started = false
    showAddDialog.value = false
    ElMessage.success('登录成功')
    fetchAccounts()
  })
  es.addEventListener('error', () => {
    loginState.value.messages.push({ text: '❌ 登录失败', type: 'error' })
    es.close()
    loginState.value.started = false
  })
}

const onCookieUploaded = (res) => {
  if (res.code === 200) {
    ElMessage.success('Cookie 导入成功')
    showImportDialog.value = false
    fetchAccounts()
  } else {
    ElMessage.error(res.msg || '导入失败')
  }
}

onMounted(fetchAccounts)
</script>

<style lang="scss" scoped>
@use '@/styles/variables.scss' as *;

.account-management {
  max-width: 1400px;
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

// ── Stats Bar ──
.stats-bar {
  margin-bottom: 20px;
}

.mini-stat {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 16px 12px;

  .val {
    font-size: 24px;
    font-weight: 700;
    color: $text-primary;
  }

  .lbl {
    font-size: 12px;
    color: $text-muted;
    margin-top: 4px;
  }

  &.good .val { color: $success-color; text-shadow: 0 0 12px rgba(0, 255, 136, 0.3); }
  &.warn .val { color: $danger-color; text-shadow: 0 0 12px rgba(255, 59, 92, 0.3); }
}

// ── Toolbar ──
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 20px;
  margin-bottom: 20px;
  gap: 16px;
  flex-wrap: wrap;

  .search-input { max-width: 280px; }
  .toolbar-actions { display: flex; gap: 8px; }
}

// ── Table ──
.table-wrapper {
  padding: 4px;
  overflow: hidden;
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;

  .el-table {
    flex: 1;
    min-height: 0;
  }
}

// ── Login Progress ──
.login-progress {
  margin-top: 16px;
}

.login-log {
  max-height: 200px;
  overflow-y: auto;
  padding: 12px 16px;
  font-size: 13px;

  p {
    margin: 4px 0;
    line-height: 1.6;

    &.success { color: $success-color; }
    &.error { color: $danger-color; }
    &.info { color: $text-secondary; }
  }
}
</style>
