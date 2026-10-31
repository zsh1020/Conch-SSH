<template>
  <Teleport to="body">
    <Transition name="preview-fade">
      <div
        v-if="state.visible"
        class="pv-overlay"
        style="--wails-draggable: drag"
        @keydown.esc.stop="$emit('close')"
      >
        <div class="pv-modal" :style="boxStyle" style="--wails-draggable: no-drag">
          <!-- 头部 -->
          <div class="pv-header" style="cursor: grab" @mousedown.prevent="startDrag">
            <img v-if="fileIcon({ name: state.name }).startsWith('/')" :src="fileIcon({ name: state.name })" class="pv-header-icon" alt="" />
            <span v-else class="pv-header-icon pv-header-icon-emoji">{{ fileIcon({ name: state.name }) }}</span>
            <div class="pv-header-meta">
              <span class="pv-header-name" :title="state.name">{{ state.name }}</span>
              <span v-if="state.size > 0" class="pv-header-size">{{ fmtSize(state.size) }}</span>
            </div>
            <button class="pv-close-btn" title="关闭 (Esc)" style="cursor: default" @click="$emit('close')">✕</button>
          </div>

          <!-- 主体 -->
          <div class="pv-body">
            <div v-if="state.loading" class="pv-loading">
              <div class="pv-spinner" />
              <span>加载中…</span>
            </div>
            <div v-else-if="state.error" class="pv-error">
              <span class="pv-error-icon">⚠️</span>
              <span>{{ state.error }}</span>
            </div>
            <img
              v-else-if="state.type === 'image'"
              :src="state.dataUrl"
              class="pv-image"
              alt=""
            />
            <pre v-else-if="state.type === 'text'" class="pv-text">{{ state.text }}</pre>
            <div v-else class="pv-binary">
              <img v-if="fileIcon({ name: state.name }).startsWith('/')" :src="fileIcon({ name: state.name })" class="pv-binary-icon" alt="" />
              <span v-else class="pv-binary-icon pv-binary-icon-emoji">{{ fileIcon({ name: state.name }) }}</span>
              <div class="pv-binary-name">{{ state.name }}</div>
              <div class="pv-binary-size">{{ fmtSize(state.size) }}</div>
              <div class="pv-binary-hint">此文件类型暂不支持预览</div>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { useDraggable } from '../composables/useDraggable'

const { boxStyle, startDrag } = useDraggable()

export interface PreviewState {
  visible: boolean
  loading: boolean
  error: string
  name: string
  size: number
  type: 'text' | 'image' | 'binary' | ''
  text: string
  dataUrl: string
}

defineProps<{
  state: PreviewState
  fileIcon: (f: { name: string; is_dir?: boolean }) => string
  fmtSize: (bytes: number) => string
}>()

defineEmits<{ (e: 'close'): void }>()
</script>

<style>
.pv-overlay {
  position: fixed;
  inset: 0;
  z-index: 8000;
  background: rgba(0, 0, 0, 0.72);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}

.pv-modal {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 12px;
  width: min(860px, 100%);
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 32px 80px rgba(0, 0, 0, 0.8);
  overflow: hidden;
  animation: pv-pop 0.18s ease;
}

@keyframes pv-pop {
  from { transform: scale(0.94); opacity: 0; }
  to   { transform: scale(1);    opacity: 1; }
}

.pv-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 18px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
  background: var(--bg-panel);
}

.pv-header-icon { width: 20px; height: 20px; flex-shrink: 0; filter: brightness(1.1); }
.pv-header-icon-emoji { width: auto; height: auto; font-size: 20px; filter: none; line-height: 1; }

.pv-header-meta {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.pv-header-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pv-header-size { font-size: 11px; color: var(--text-muted); }

.pv-close-btn {
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text-muted);
  font-size: 12px;
  padding: 4px 9px;
  cursor: pointer;
  flex-shrink: 0;
  transition: color 0.15s, background 0.15s;
}
.pv-close-btn:hover { color: var(--text-primary); background: rgba(255,255,255,0.06); }

.pv-body {
  flex: 1;
  overflow: auto;
  display: flex;
  flex-direction: column;
}

.pv-loading,
.pv-error {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 40px;
  color: var(--text-muted);
  font-size: 13px;
}

.pv-error { color: var(--danger); }
.pv-error-icon { font-size: 28px; }

.pv-spinner {
  width: 32px;
  height: 32px;
  border: 3px solid var(--border);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: pv-spin 0.7s linear infinite;
}
@keyframes pv-spin { to { transform: rotate(360deg); } }

.pv-image {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  display: block;
  margin: auto;
  padding: 16px;
}

.pv-text {
  flex: 1;
  margin: 0;
  padding: 16px 20px;
  font-family: 'Cascadia Code', 'JetBrains Mono', Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  color: var(--text-primary);
  white-space: pre-wrap;
  word-break: break-all;
  tab-size: 4;
}

.pv-binary {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 48px;
  color: var(--text-muted);
  text-align: center;
}

.pv-binary-icon { width: 56px; height: 56px; filter: brightness(1.1); }
.pv-binary-icon-emoji { width: auto; height: auto; font-size: 56px; filter: none; line-height: 1; }
.pv-binary-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
  word-break: break-all;
}
.pv-binary-size { font-size: 12px; }
.pv-binary-hint { font-size: 12px; opacity: 0.6; margin-top: 4px; }

.preview-fade-enter-active,
.preview-fade-leave-active { transition: opacity 0.18s; }
.preview-fade-enter-from,
.preview-fade-leave-to     { opacity: 0; }
</style>
