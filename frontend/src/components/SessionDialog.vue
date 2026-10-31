<template>
  <div class="dialog-overlay" style="--wails-draggable: drag">
    <div class="dialog" :style="boxStyle" style="--wails-draggable: no-drag">
      <div class="dialog-header" style="cursor: grab" @mousedown.prevent="startDrag">
        <h2>{{ isEdit ? '编辑会话' : '新建会话' }}</h2>
        <button class="icon-btn btn-ghost" style="cursor: default" @click="$emit('close')">✕</button>
      </div>

      <form class="dialog-body" @submit.prevent="onSubmit">
        <!-- 基本信息 -->
        <fieldset>
          <legend>基本信息</legend>
          <div class="form-grid">
            <div class="form-field span-2">
              <label>会话名称</label>
              <input v-model="form.name" placeholder="如：生产 Web 服务器" />
            </div>
            <div class="form-field">
              <label>分组</label>
              <input
                v-model="form.group"
                placeholder="默认"
                list="group-list"
              />
              <datalist id="group-list">
                <option v-for="g in existingGroups" :key="g" :value="g" />
              </datalist>
            </div>
            <div class="form-field">
              <label>标签 <span class="hint">（逗号分隔）</span></label>
              <input v-model="tagsInput" placeholder="prod, linux, web" />
            </div>
          </div>
        </fieldset>

        <!-- 连接参数 -->
        <fieldset>
          <legend>连接参数</legend>
          <div class="form-grid">
            <div class="form-field span-2">
              <label>主机 <span class="required">*</span></label>
              <input v-model="form.host" placeholder="hostname 或 IP" required />
            </div>
            <div class="form-field">
              <label>端口</label>
              <input v-model.number="form.port" type="number" min="1" max="65535" placeholder="22" />
            </div>
            <div class="form-field">
              <label>用户名 <span class="required">*</span></label>
              <input v-model="form.username" placeholder="root" required />
            </div>
          </div>
        </fieldset>

        <!-- 认证方式 -->
        <fieldset>
          <legend>认证方式</legend>
          <div class="auth-tabs">
            <button
              v-for="m in authModes"
              :key="m.value"
              type="button"
              class="auth-tab"
              :class="{ active: form.auth_type === m.value }"
              @click="form.auth_type = m.value"
            >
              {{ m.label }}
            </button>
          </div>

          <div class="form-grid" style="margin-top: 12px;">
            <template v-if="form.auth_type === 'password'">
              <div class="form-field span-2">
                <label>密码</label>
                <div class="input-with-toggle">
                  <input
                    v-model="form.password"
                    :type="showPwd ? 'text' : 'password'"
                    placeholder="SSH 密码（留空则连接时询问）"
                  />
                  <button type="button" class="toggle-btn" @click="showPwd = !showPwd">
                    {{ showPwd ? '隐藏' : '显示' }}
                  </button>
                </div>
              </div>
            </template>

            <template v-else-if="form.auth_type === 'key'">
              <div class="form-field span-2">
                <label>私钥路径</label>
                <input v-model="form.identity_file" placeholder="~/.ssh/id_ed25519" />
              </div>
              <div class="form-field span-2">
                <label>私钥密码短语 <span class="hint">（若有加密）</span></label>
                <input v-model="form.password" type="password" placeholder="留空则无加密" />
              </div>
            </template>

            <template v-else-if="form.auth_type === 'agent'">
              <div class="form-field span-2">
                <p class="info-text">使用系统 SSH Agent（需设置 SSH_AUTH_SOCK 环境变量）</p>
              </div>
            </template>
          </div>
        </fieldset>

        <!-- 高级选项 -->
        <fieldset>
          <legend>
            <button type="button" class="legend-toggle" @click="showAdvanced = !showAdvanced">
              高级选项 {{ showAdvanced ? '▲' : '▼' }}
            </button>
          </legend>
          <div v-if="showAdvanced" class="form-grid">
            <div class="form-field span-2">
              <label>跳板机 <span class="hint">（ProxyJump，如 user@bastion:22）</span></label>
              <input v-model="form.proxy_jump" placeholder="user@bastion.example.com:22" />
            </div>
          </div>
        </fieldset>

        <div v-if="errorMsg" class="error-msg">{{ errorMsg }}</div>

        <div class="dialog-footer">
          <button type="button" class="btn-ghost" @click="$emit('close')">取消</button>
          <button type="submit" class="btn-primary" :disabled="saving">
            {{ saving ? '保存中…' : (isEdit ? '保存修改' : '创建会话') }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, computed, watch } from 'vue'
import { useSessionsStore, type Session } from '../stores/sessions'
import { useDraggable } from '../composables/useDraggable'

const { boxStyle, startDrag } = useDraggable()
import { store as wailsStore } from '../../wailsjs/go/models'

const props = defineProps<{
  session?: Session   // 传入则为编辑模式
  existingGroups: string[]
}>()

const emit = defineEmits<{
  close: []
  saved: [sess: Session]
}>()

const store = useSessionsStore()

const isEdit = computed(() => !!props.session?.id)
const showPwd = ref(false)
const showAdvanced = ref(false)
const saving = ref(false)
const errorMsg = ref('')

const authModes = [
  { value: 'password', label: '密码' },
  { value: 'key',      label: '私钥' },
  { value: 'agent',    label: 'SSH Agent' },
]

const form = reactive<Partial<Session> & { auth_type: string }>({
  id: '',
  name: '',
  group: '',
  host: '',
  port: 22,
  username: 'root',
  auth_type: 'password',
  password: '',
  identity_file: '',
  proxy_jump: '',
  tags: [],
})

const tagsInput = ref('')

// 编辑模式：回显已有配置
watch(() => props.session, (s) => {
  if (!s) return
  Object.assign(form, s)
  tagsInput.value = (s.tags ?? []).join(', ')
  if (s.proxy_jump) showAdvanced.value = true
}, { immediate: true })

async function onSubmit() {
  errorMsg.value = ''
  saving.value = true
  try {
    const sess: Session = wailsStore.Session.createFrom({
      id:            form.id ?? '',
      name:          form.name || `${form.username}@${form.host}`,
      group:         form.group || '',
      host:          form.host ?? '',
      port:          form.port || 22,
      username:      form.username ?? '',
      auth_type:     form.auth_type,
      password:      form.password ?? '',
      identity_file: form.identity_file ?? '',
      proxy_jump:    form.proxy_jump ?? '',
      tags:          tagsInput.value.split(',').map((t: string) => t.trim()).filter(Boolean),
      last_used:     form.last_used ?? null,
    })
    await store.save(sess)
    emit('saved', sess)
    emit('close')
  } catch (e: any) {
    errorMsg.value = String(e)
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.dialog-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.65);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 200;
  backdrop-filter: blur(2px);
}

.dialog {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 10px;
  width: 520px;
  max-height: 88vh;
  overflow-y: auto;
  box-shadow: 0 24px 48px rgba(0, 0, 0, 0.6);
}

.dialog-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px 12px;
  border-bottom: 1px solid var(--border);
  position: sticky;
  top: 0;
  background: var(--bg-secondary);
  z-index: 1;
}

.dialog-header h2 {
  font-size: 15px;
  font-weight: 600;
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
  gap: 16px;
}

fieldset {
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 12px 14px 14px;
}

legend {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  padding: 0 6px;
}

.legend-toggle {
  background: none;
  border: none;
  padding: 0;
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  cursor: pointer;
}

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.form-field.span-2 {
  grid-column: span 2;
}

.form-field label {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
}

.required { color: var(--danger); }
.hint { color: var(--text-muted); font-weight: 400; font-size: 11px; }

.auth-tabs {
  display: flex;
  gap: 4px;
  background: var(--bg-primary);
  border-radius: 6px;
  padding: 3px;
}

.auth-tab {
  flex: 1;
  background: transparent;
  border: none;
  border-radius: 4px;
  padding: 5px 8px;
  font-size: 12px;
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.15s;
}

.auth-tab.active {
  background: var(--bg-secondary);
  color: var(--text-primary);
  box-shadow: 0 1px 3px rgba(0,0,0,0.3);
}

.input-with-toggle {
  display: flex;
  gap: 6px;
}

.input-with-toggle input {
  flex: 1;
}

.toggle-btn {
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 0 10px;
  font-size: 12px;
  color: var(--text-muted);
  cursor: pointer;
  white-space: nowrap;
}

.info-text {
  font-size: 12px;
  color: var(--text-muted);
  padding: 6px 0;
  line-height: 1.6;
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
  padding-top: 4px;
}
</style>
