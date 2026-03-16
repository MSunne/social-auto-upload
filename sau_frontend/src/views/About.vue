<template>
  <div class="about fade-in">
    <div class="about-hero glass-card">
      <div class="hero-glow"></div>
      <div class="hero-content">
        <div class="logo-large">
          <el-icon :size="42"><Monitor /></el-icon>
        </div>
        <h1 class="hero-title">OmniBull</h1>
        <p class="hero-subtitle">自媒体自动化运营系统</p>
        <p class="hero-version">social-auto-upload · v2.0</p>
      </div>
    </div>

    <el-row :gutter="16" style="margin-top: 20px">
      <el-col :span="12">
        <div class="about-section glass-card">
          <h3><el-icon><Star /></el-icon> 核心功能</h3>
          <ul class="feature-list">
            <li>多平台账号管理与登录状态维护</li>
            <li>视频素材上传与管理</li>
            <li>一键多平台发布（抖音 / 快手 / 视频号 / 小红书）</li>
            <li>定时发布与批量发布</li>
            <li>Cookie 导入导出</li>
            <li>OmniDrive 云端同步</li>
          </ul>
        </div>
      </el-col>

      <el-col :span="12">
        <div class="about-section glass-card">
          <h3><el-icon><Setting /></el-icon> 技术栈</h3>
          <div class="tech-tags">
            <span class="tech-tag violet">Vue 3</span>
            <span class="tech-tag cyan">Element Plus</span>
            <span class="tech-tag pink">Pinia</span>
            <span class="tech-tag green">Flask</span>
            <span class="tech-tag violet">Playwright</span>
            <span class="tech-tag cyan">SQLite</span>
          </div>

          <h3 style="margin-top: 20px"><el-icon><Platform /></el-icon> 支持平台</h3>
          <div class="platform-tags">
            <el-tag type="danger" effect="dark" size="large">抖音</el-tag>
            <el-tag type="success" effect="dark" size="large">快手</el-tag>
            <el-tag type="warning" effect="dark" size="large">视频号</el-tag>
            <el-tag effect="dark" size="large">小红书</el-tag>
          </div>
        </div>
      </el-col>
    </el-row>

    <div class="about-footer">
      <p>Powered by <span class="text-glow">OmniBull Engine</span> · MIT License</p>
    </div>
  </div>
</template>

<script setup>
// About page
</script>

<style lang="scss" scoped>
@use '@/styles/variables.scss' as *;

.about {
  max-width: 800px;
  margin: 0 auto;
}

// ── Hero ──
.about-hero {
  position: relative;
  text-align: center;
  padding: 48px 24px;
  overflow: hidden;

  .hero-glow {
    position: absolute;
    top: -40%;
    left: 50%;
    transform: translateX(-50%);
    width: 300px;
    height: 200px;
    background: radial-gradient(ellipse, rgba(177, 73, 255, 0.25), transparent);
    filter: blur(60px);
    pointer-events: none;
  }

  .hero-content {
    position: relative;
    z-index: 1;
  }

  .logo-large {
    width: 72px;
    height: 72px;
    border-radius: 20px;
    background: $accent-gradient;
    display: flex;
    align-items: center;
    justify-content: center;
    margin: 0 auto 16px;
    box-shadow: 0 0 30px $accent-glow;

    .el-icon { color: #fff; }
  }

  .hero-title {
    font-size: 28px;
    font-weight: 700;
    color: $text-primary;
    margin: 0 0 6px;
    text-shadow: 0 0 20px rgba(177, 73, 255, 0.3);
  }

  .hero-subtitle {
    font-size: 15px;
    color: $text-secondary;
    margin: 0 0 4px;
  }

  .hero-version {
    font-size: 12px;
    color: $text-muted;
    margin: 0;
    font-family: 'JetBrains Mono', monospace;
  }
}

// ── Sections ──
.about-section {
  padding: 24px;
  height: 100%;

  h3 {
    font-size: 15px;
    font-weight: 600;
    color: $text-primary;
    margin: 0 0 16px;
    display: flex;
    align-items: center;
    gap: 8px;

    .el-icon { color: $accent-color; }
  }

  .feature-list {
    list-style: none;
    padding: 0;
    margin: 0;

    li {
      padding: 8px 0;
      color: $text-secondary;
      font-size: 13px;
      border-bottom: 1px solid $border-color;
      display: flex;
      align-items: center;
      gap: 8px;

      &::before {
        content: '›';
        color: $cyan-color;
        font-weight: 700;
        font-size: 16px;
      }

      &:last-child { border-bottom: none; }
    }
  }
}

// ── Tech Tags — neon pills ──
.tech-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.tech-tag {
  padding: 4px 14px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;

  &.violet {
    background: rgba(177, 73, 255, 0.12);
    color: $accent-strong;
    border: 1px solid rgba(177, 73, 255, 0.2);
  }
  &.cyan {
    background: rgba(0, 245, 212, 0.08);
    color: $cyan-color;
    border: 1px solid rgba(0, 245, 212, 0.18);
  }
  &.pink {
    background: rgba(255, 45, 120, 0.08);
    color: $pink-color;
    border: 1px solid rgba(255, 45, 120, 0.18);
  }
  &.green {
    background: rgba(0, 255, 136, 0.08);
    color: $success-color;
    border: 1px solid rgba(0, 255, 136, 0.18);
  }
}

.platform-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

// ── Footer ──
.about-footer {
  text-align: center;
  margin-top: 28px;
  padding: 16px 0;

  p {
    font-size: 13px;
    color: $text-muted;
    margin: 0;
  }
}
</style>
