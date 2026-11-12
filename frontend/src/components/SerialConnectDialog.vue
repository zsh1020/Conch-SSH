<template>
  <div class="dialog-overlay" style="--wails-draggable: drag">
    <div class="dialog" :style="boxStyle" style="--wails-draggable: no-drag">
      <div class="dialog-header" style="cursor: grab" @mousedown.prevent="startDrag">
        <h2>串口连接</h2>
        <button class="btn-ghost icon-btn" style="cursor: default" @click="$emit('cancel')">✕</button>
      </div>

      <form class="dialog-body" @submit.prevent="onConnect">
        <div class="form-row">
          <label>串口</label>
          <div style="display: flex; gap: 8px">
            <select v-model="form.port" style="flex: 1" required>
              <option v-if="ports.length === 0" value="" disabled>未检测到串口设备</option>
              <option v-for="p in ports" :key="p.name" :value="p.name">
                {{ p.description ? `${p.name} · ${p.description}` : p.name }}
              </option>
            </select>
            <button type="button" class="btn-ghost" title="刷新串口列表" @click="loadPorts">↻</button>
          </div>
        </div>

        <div class="form-row form-row-half">
          <div>
            <label>波特率</label>
            <select v-model.number="form.baud_rate">
              <option v-for="b in baudRates" :key="b" :value="b">{{ b }}</option>
            </select>
          </div>
          <div>
            <label>数据位</label>
            <select v-model.number="form.data_bits">
              <option :value="8">8</option>
              <option :value="7">7</option>
              <option :value="6">6</option>
              <option :value="5">5</option>
            </select>
          </div>
        </div>

        <div class="form-row form-row-half">
          <div>
            <label>校验位</label>
            <select v-model="form.parity">
              <option value="none">无 (None)</option>
              <option value="odd">奇校验 (Odd)</option>
              <option value="even">偶校验 (Even)</option>
            </select>
          </div>
          <div>
            <label>停止位</label>
            <select v-model="form.stop_bits">
              <option value="1">1</option>
              <option value="1.5">1.5</option>
              <option value="2">2</option>
            </select>
          </div>
        </div>

        <div v-if="error" class="error-msg">{{ error }}</div>

        <div class="dialog-footer">
          <button type="button" class="btn-ghost" @click="$emit('cancel')">取消</button>
          <button type="submit" class="btn-primary" :disabled="connecting || !form.port">
            {{ connecting ? '连接中…' : '连接' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { SerialListPorts, SerialConnect } from '../../wailsjs/go/main/App'
import type { serial } from '../../wailsjs/go/models'
import { useDraggable } from '../composables/useDraggable'
import type { SerialConnectRequest } from '../stores/tabs'

const { boxStyle, startDrag } = useDraggable()

const emit = defineEmits<{
  connected: [connId: string, label: string, req: SerialConnectRequest]
  cancel: []
}>()

const baudRates = [9600, 19200, 38400, 57600, 115200, 230400, 460800, 921600]

const form = reactive<SerialConnectRequest>({
  port: '',
  baud_rate: 115200,
  data_bits: 8,
  parity: 'none',
  stop_bits: '1',
})

const ports = ref<serial.PortInfo[]>([])
const error = ref('')
const connecting = ref(false)

async function loadPorts() {
  try {
    ports.value = await SerialListPorts()
    const names = ports.value.map(p => p.name)
    if (names.length > 0 && !names.includes(form.port)) {
      form.port = names[0]
    }
  } catch (e: any) {
    error.value = '枚举串口失败：' + String(e)
  }
}

onMounted(loadPorts)

async function onConnect() {
  if (connecting.value || !form.port) return
  connecting.value = true
  error.value = ''
  const req: SerialConnectRequest = { ...form }
  try {
    const connId = await SerialConnect(req)
    emit('connected', connId, `${form.port} @ ${form.baud_rate}`, req)
  } catch (e: any) {
    error.value = '连接失败：' + String(e)
  } finally {
    connecting.value = false
  }
}
</script>

<style scoped>
.dialog-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  backdrop-filter: blur(2px);
}

.dialog {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 10px;
  width: 440px;
  box-shadow: 0 24px 48px rgba(0, 0, 0, 0.5);
}

.dialog-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px 12px;
  border-bottom: 1px solid var(--border);
}

.dialog-header h2 {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

.icon-btn {
  width: 28px;
  height: 28px;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  border-radius: 4px;
}

.dialog-body {
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.form-row {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.form-row label {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
}

.form-row-half {
  flex-direction: row;
  gap: 12px;
}

.form-row-half > div {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.error-msg {
  background: rgba(248, 81, 73, 0.1);
  border: 1px solid rgba(248, 81, 73, 0.3);
  border-radius: 6px;
  padding: 8px 12px;
  font-size: 12px;
  color: var(--danger);
}

.dialog-footer {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  margin-top: 4px;
}
</style>
