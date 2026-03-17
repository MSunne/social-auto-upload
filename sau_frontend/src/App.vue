<template>
  <div id="app" class="dark">
    <el-container>
      <!-- ════ Sidebar ════ -->
      <el-aside :width="sidebarCollapsed ? '64px' : '220px'">
        <div class="sidebar">
          <!-- Logo -->
          <div class="sidebar-logo">
            <div class="logo-icon">
              <el-icon :size="22"><Monitor /></el-icon>
            </div>
            <transition name="fade">
              <span v-show="!sidebarCollapsed" class="logo-text">OmniBull</span>
            </transition>
          </div>

          <!-- Navigation -->
          <el-menu
            :router="true"
            :default-active="activeMenu"
            :collapse="sidebarCollapsed"
            class="sidebar-menu"
          >
            <el-menu-item index="/">
              <el-icon><HomeFilled /></el-icon>
              <span>仪表盘</span>
            </el-menu-item>
            <el-menu-item index="/account-management">
              <el-icon><User /></el-icon>
              <span>账号管理</span>
            </el-menu-item>
            <el-menu-item index="/material-management">
              <el-icon><Picture /></el-icon>
              <span>素材管理</span>
            </el-menu-item>
            <el-menu-item index="/publish-center">
              <el-icon><Upload /></el-icon>
              <span>发布中心</span>
            </el-menu-item>
            <el-menu-item index="/system-status">
              <el-icon><DataLine /></el-icon>
              <span>系统状态</span>
            </el-menu-item>
            <el-menu-item index="/about">
              <el-icon><InfoFilled /></el-icon>
              <span>关于</span>
            </el-menu-item>
          </el-menu>

          <!-- Collapse Toggle -->
          <div class="sidebar-footer" @click="appStore.toggleSidebar()">
            <el-icon :size="18">
              <component :is="sidebarCollapsed ? 'Expand' : 'Fold'" />
            </el-icon>
            <transition name="fade">
              <span v-show="!sidebarCollapsed" class="collapse-text">收起菜单</span>
            </transition>
          </div>
        </div>
      </el-aside>

      <!-- ════ Main Area ════ -->
      <el-container>
        <!-- Header -->
        <el-header>
          <div class="header-content">
            <div class="header-left">
              <h2 class="page-title">{{ currentTitle }}</h2>
            </div>
            <div class="header-right">
              <div class="connection-dot" :class="{ online: isOnline }">
                <span class="dot" :class="{ 'pulse-online': isOnline }"></span>
                <span class="label">{{ isOnline ? '后端已连接' : '后端未连接' }}</span>
              </div>
            </div>
          </div>
        </el-header>

        <!-- Content -->
        <el-main>
          <router-view v-slot="{ Component }">
            <transition name="page-fade" mode="out-in">
              <component :is="Component" />
            </transition>
          </router-view>
        </el-main>
      </el-container>
    </el-container>
  </div>
</template>

<script setup>
import { computed, ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores/app'
import axios from 'axios'

const route = useRoute()
const appStore = useAppStore()
const isOnline = ref(false)

const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const activeMenu = computed(() => route.path)
const currentTitle = computed(() => route.meta.title || 'OmniBull')

const checkBackend = async () => {
  try {
    const baseUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:5409'
    await axios.get(`${baseUrl}/getAccounts`, { timeout: 3000 })
    isOnline.value = true
  } catch {
    isOnline.value = false
  }
}

onMounted(() => {
  checkBackend()
  setInterval(checkBackend, 30000)
})
</script>

<style lang="scss" scoped>
@use '@/styles/variables.scss' as *;

#app {
  min-height: 100vh;
  height: 100vh;
  background: transparent; // body::before handles animated BG
}

// Outermost container: sidebar + right area
.el-container {
  height: 100vh;

  // Inner container (header + main): must stretch to fill remaining width
  > .el-container {
    display: flex;
    flex-direction: column;
    flex: 1;
    overflow: hidden;
    height: 100vh;
  }
}

// ── Sidebar — Deep Glass ──
.el-aside {
  background: $bg-sidebar;
  height: 100vh;
  overflow: hidden;
  transition: width $transition-base;
  border-right: 1px solid $border-color;
  position: relative;
  backdrop-filter: $glass-blur;
  -webkit-backdrop-filter: $glass-blur;

  .sidebar {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
}

.sidebar-logo {
  height: 60px;
  display: flex;
  align-items: center;
  padding: 0 16px;
  border-bottom: 1px solid rgba(139, 92, 246, 0.08);
  gap: 12px;
  flex-shrink: 0;

  .logo-icon {
    width: 36px;
    height: 36px;
    border-radius: 10px;
    background: $accent-gradient;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    box-shadow: 0 0 16px $accent-glow;

    .el-icon { color: #fff; }
  }

  .logo-text {
    font-size: 17px;
    font-weight: 700;
    color: $text-primary;
    white-space: nowrap;
    letter-spacing: 0.5px;
    text-shadow: 0 0 20px rgba(177, 73, 255, 0.3);
  }
}

.sidebar-menu {
  flex: 1;
  border-right: none !important;
  padding: 12px 8px;
  overflow-y: auto;

  --el-menu-bg-color: transparent;
  --el-menu-text-color: #{$text-secondary};
  --el-menu-active-color: #{$accent-color};
  --el-menu-hover-bg-color: #{$bg-hover};

  .el-menu-item {
    border-radius: 10px;
    margin-bottom: 4px;
    height: 44px;
    line-height: 44px;
    transition: all $transition-base;

    .el-icon {
      font-size: 18px;
      margin-right: 10px;
    }

    &:hover {
      background: $bg-hover;
    }

    &.is-active {
      background: rgba(177, 73, 255, 0.15);
      color: $accent-color;
      font-weight: 600;
      box-shadow: 0 0 20px rgba(177, 73, 255, 0.08);

      .el-icon { color: $accent-color; }
    }
  }
}

.sidebar-footer {
  padding: 16px;
  border-top: 1px solid rgba(139, 92, 246, 0.08);
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  color: $text-muted;
  transition: color $transition-fast;
  flex-shrink: 0;

  &:hover { color: $accent-color; }

  .collapse-text {
    font-size: 13px;
    white-space: nowrap;
  }
}

// ── Header — Glass Bar ──
.el-header {
  background: $bg-surface-alt;
  border-bottom: 1px solid $border-color;
  backdrop-filter: $glass-blur;
  -webkit-backdrop-filter: $glass-blur;
  padding: 0;
  height: $header-height;

  .header-content {
    display: flex;
    justify-content: space-between;
    align-items: center;
    height: 100%;
    padding: 0 24px;
  }

  .page-title {
    font-size: 18px;
    font-weight: 600;
    color: $text-primary;
    margin: 0;
  }

  .connection-dot {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: $text-muted;

    .dot {
      width: 8px;
      height: 8px;
      border-radius: 50%;
      background: $danger-color;
      transition: all $transition-base;
    }

    &.online .dot {
      background: $success-color;
      box-shadow: 0 0 10px rgba(0, 255, 136, 0.5);
    }

    .label { white-space: nowrap; }
  }
}

// ── Main ──
.el-main {
  background: transparent;
  padding: 24px;
  overflow-y: auto;
  flex: 1;
  min-height: 0; // allow flex child to shrink below content size
}

// ── Transitions ──
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from, .fade-leave-to { opacity: 0; }

.page-fade-enter-active,
.page-fade-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}
.page-fade-enter-from {
  opacity: 0;
  transform: translateY(6px);
}
.page-fade-leave-to { opacity: 0; }
</style>
