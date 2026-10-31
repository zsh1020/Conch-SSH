<template>
  <div class="dialog-overlay" style="--wails-draggable: drag">
    <div class="dialog" :style="boxStyle" style="--wails-draggable: no-drag">
      <div class="dialog-header" style="cursor: grab" @mousedown.prevent="startDrag">
        <h2>SSH 连接</h2>
        <button class="btn-ghost icon-btn" style="cursor: default" @click="$emit('cancel')">✕</button>
      </div>

      <form class="dialog-body" @submit.prevent="onConnect">
        <div class="form-row">
          <label>主机</label>
          <input
            v-model="form.host"
            placeholder="hostname 或 IP"
            required
            autofocus
          />
        </div>

        <div class="form-row form-row-half">
          <div>
            <label>端口</label>
            <input v-model.number="form.port" type="number" min="1" max="65535" placeholder="22" />
          </div>
          <div>
            <label>用户名</label>
            <input v-model="form.username" placeholder="root" required />
          </div>
        </div>

        <div class="form-row">
          <label>认证方式</label>
          <select v-model="form.authType">
            <option value="password">密码</option>
            <option value="key">私钥文件</option>
            <option value="agent">SSH Agent</option>
          </select>
        </div>

        <div v-if="form.authType === 'password'" class="form-row">
          <label>密码</label>
          <input v-model="form.password" type="password" placeholder="SSH 密码" />
        </div>

        <div v-if="form.authType === 'key'" class="form-row">
          <label>私钥路径</label>
          <input v-model="form.identityFile" placeholder="~/.ssh/id_ed25519" />
        </div>

        <div v-if="error" class="error-msg">{{ error }}</div>

        <div class="dialog-footer">
          <button type="button" class="btn-ghost" @click="$emit('cancel')">取消</button>
          <button type="submit" class="btn-primary" :disabled="connecting">
            {{ connecting ? '连接中…' : '连接' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { Connect } from '../../wailsjs/go/main/App'
import { useDraggable } from '../composables/useDraggable'

const { boxStyle, startDrag } = useDraggable()

const emit = defineEmits<{
  connected: [connId: string, label: string]
  cancel: []
}>()

const form = reactive({
  host: '',
  port: 22,
  username: 'root',
  authType: 'password',
  password: '',
  identityFile: '',
})

const connecting = ref(false)
const error = ref('')

async function onConnect() {
  error.value = ''
  connecting.value = true
  try {
    const connId = await Connect(
      {
        host: form.host,
        port: form.port || 22,
        username: form.username,
        auth_type: form.authType,
        password: form.password,
        identity_file: form.identityFile,
      },
      80,
      24
    )
    const label = `${form.username}@${form.host}`
    emit('connected', connId, label)
  } catch (e: any) {
    error.value = String(e)
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
