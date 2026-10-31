<template>
  <div v-if="modelValue && tabsStore.tabs.length > 0" class="broadcast-bar">
    <span class="bc-label">广播:</span>
    <div class="bc-targets">
      <label
        v-for="tab in tabsStore.tabs"
        :key="tab.connId"
        class="bc-check"
      >
        <input
          type="checkbox"
          :value="tab.connId"
          v-model="broadcastTargets"
        />
        <span>{{ tab.label }}</span>
      </label>
    </div>
    <input
      v-model="broadcastCmd"
      class="bc-input"
      placeholder="输入命令，按 Enter 发送…"
      @keydown.enter.prevent="sendBroadcast"
    />
    <button class="bc-send" @click="sendBroadcast">发送</button>
    <button class="bc-close" @click="$emit('update:modelValue', false)">✕</button>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { SendInput } from '../../wailsjs/go/main/App'
import { useTabsStore } from '../stores/tabs'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>()

const tabsStore = useTabsStore()

const broadcastCmd     = ref('')
const broadcastTargets = ref<string[]>([])

watch(() => props.modelValue, (on) => {
  if (on) {
    broadcastTargets.value = tabsStore.tabs
      .filter(t => t.status === 'connected')
      .map(t => t.connId)
  }
})

async function sendBroadcast() {
  const cmd = broadcastCmd.value.trim()
  if (!cmd || broadcastTargets.value.length === 0) return
  await Promise.allSettled(
    broadcastTargets.value.map(id => SendInput(id, cmd + '\n'))
  )
  broadcastCmd.value = ''
}
</script>

<style>
.broadcast-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 10px;
  background: rgba(88,166,255,0.08);
  border-bottom: 1px solid rgba(88,166,255,0.3);
  flex-shrink: 0;
  flex-wrap: wrap;
}

.bc-label {
  font-size: 11px;
  color: var(--accent);
  font-weight: 600;
  flex-shrink: 0;
}

.bc-targets {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.bc-check {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: var(--text-primary);
  cursor: pointer;
}

.bc-check input[type="checkbox"] {
  width: auto;
  padding: 0;
  border: none;
  background: transparent;
  cursor: pointer;
}

.bc-input {
  flex: 1;
  min-width: 160px;
  font-size: 12px;
  padding: 4px 8px;
  height: auto;
}

.bc-send {
  background: var(--accent);
  border: none;
  border-radius: 4px;
  color: #fff;
  font-size: 12px;
  padding: 4px 12px;
  cursor: pointer;
  flex-shrink: 0;
}
.bc-send:hover { filter: brightness(1.1); }

.bc-close {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 12px;
  padding: 2px 4px;
  cursor: pointer;
  border-radius: 3px;
  flex-shrink: 0;
}
.bc-close:hover { color: var(--danger); }
</style>
