<template>
  <div v-if="modelValue" class="help-overlay">
    <div class="help-panel">
      <div class="help-header">
        <span class="help-title">⌨ 快捷键速查</span>
        <button class="help-close" @click="$emit('update:modelValue', false)">✕</button>
      </div>
      <div class="help-body">
        <div class="help-section">
          <div class="help-section-title">标签页</div>
          <div class="help-row"><kbd>Ctrl+T</kbd><span>新建连接</span></div>
          <div class="help-row"><kbd>Ctrl+W</kbd><span>关闭当前标签</span></div>
          <div class="help-row"><kbd>Ctrl+Tab</kbd><span>切换到下一标签</span></div>
          <div class="help-row"><kbd>Ctrl+Shift+Tab</kbd><span>切换到上一标签</span></div>
          <div class="help-row"><kbd>Ctrl+1~9</kbd><span>跳转到对应标签</span></div>
          <div class="help-row"><kbd>拖拽</kbd><span>调整标签顺序</span></div>
        </div>
        <div class="help-section">
          <div class="help-section-title">终端</div>
          <div class="help-row"><kbd>Ctrl+F</kbd><span>终端内搜索</span></div>
          <div class="help-row"><kbd>Enter</kbd><span>搜索下一个</span></div>
          <div class="help-row"><kbd>Shift+Enter</kbd><span>搜索上一个</span></div>
          <div class="help-row"><kbd>Esc</kbd><span>关闭搜索栏</span></div>
          <div class="help-row"><kbd>右键</kbd><span>复制 / 粘贴 / 全选 / 清屏</span></div>
        </div>
        <div class="help-section">
          <div class="help-section-title">侧边栏</div>
          <div class="help-row"><kbd>双击</kbd><span>连接会话</span></div>
          <div class="help-row"><kbd>右键</kbd><span>连接 / 编辑 / 收藏 / 删除</span></div>
          <div class="help-row"><kbd>拖拽边缘</kbd><span>调整侧边栏宽度</span></div>
          <div class="help-row"><kbd>★</kbd><span>收藏会话，显示在「最近」分组</span></div>
        </div>
        <div class="help-section">
          <div class="help-section-title">工具栏</div>
          <div class="help-row"><kbd>📋</kbd><span>命令片段（支持 {变量} 参数）</span></div>
          <div class="help-row"><kbd>📢</kbd><span>批量广播到多个终端</span></div>
          <div class="help-row"><kbd>📊</kbd><span>服务器 CPU/内存/网络监控</span></div>
          <div class="help-row"><kbd>🔗</kbd><span>端口转发规则管理</span></div>
        </div>
        <div class="help-section">
          <div class="help-section-title">通用</div>
          <div class="help-row"><kbd>Esc</kbd><span>关闭当前对话框 / 面板</span></div>
          <div class="help-row"><kbd>?</kbd><span>打开 / 关闭此速查面板</span></div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ modelValue: boolean }>()
defineEmits<{ (e: 'update:modelValue', v: boolean): void }>()
</script>

<style>
.help-overlay {
  position: fixed;
  inset: 0;
  z-index: 400;
  background: rgba(0,0,0,0.55);
  display: flex;
  align-items: center;
  justify-content: center;
  backdrop-filter: blur(3px);
}

.help-panel {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 12px;
  width: 520px;
  max-width: 92vw;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 28px 60px rgba(0,0,0,0.7);
  overflow: hidden;
}

.help-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.help-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-primary);
}

.help-close {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 12px;
  padding: 3px 6px;
  border-radius: 4px;
  cursor: pointer;
}
.help-close:hover { color: var(--text-primary); background: rgba(255,255,255,0.08); }

.help-body {
  overflow-y: auto;
  padding: 14px 18px;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.help-section { display: flex; flex-direction: column; gap: 6px; }

.help-section-title {
  font-size: 10px;
  font-weight: 700;
  color: var(--accent);
  text-transform: uppercase;
  letter-spacing: 0.6px;
  padding-bottom: 4px;
  border-bottom: 1px solid var(--border);
}

.help-row {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  color: var(--text-muted);
}

.help-row kbd {
  font-size: 10px;
  min-width: 120px;
  flex-shrink: 0;
  text-align: center;
  white-space: nowrap;
}

.help-row span { flex: 1; }
</style>
