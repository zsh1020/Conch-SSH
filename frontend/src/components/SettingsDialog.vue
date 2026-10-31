<template>
  <div class="sd-overlay" style="--wails-draggable: drag">
    <div class="sd-dialog" :style="boxStyle" style="--wails-draggable: no-drag">

      <div class="sd-header" style="cursor: grab" @mousedown.prevent="startDrag">
        <span class="sd-title">设置</span>
        <button class="sd-icon-btn" style="cursor: default" @click="$emit('close')">✕</button>
      </div>

      <!-- 侧边导航 + 内容区 -->
      <div class="sd-body">
        <nav class="sd-nav">
          <button
            v-for="tab in tabs"
            :key="tab.id"
            class="sd-nav-item"
            :class="{ active: activeTab === tab.id }"
            @click="activeTab = tab.id"
          >{{ tab.label }}</button>
        </nav>

        <div class="sd-content">

          <!-- ── 终端外观 ── -->
          <template v-if="activeTab === 'terminal'">
            <div class="sd-section-title">终端外观</div>

            <div class="sd-field">
              <label>配色方案</label>
              <select v-model="form.color_scheme" class="sd-select">
                <option v-for="sc in COLOR_SCHEME_OPTIONS" :key="sc.value" :value="sc.value">{{ sc.label }}</option>
              </select>
              <div class="sd-palette">
                <span
                  v-for="color in currentSchemePreview"
                  :key="color"
                  class="sd-swatch"
                  :style="{ background: color }"
                  :title="color"
                />
              </div>
            </div>

            <div class="sd-field">
              <label>字体</label>
              <select v-model="fontSelectValue" class="sd-select">
                <option v-for="f in FONT_PRESETS" :key="f.value" :value="f.value">{{ f.label }}</option>
                <option value="__custom__">自定义…</option>
              </select>
              <input
                v-if="fontSelectValue === '__custom__'"
                v-model="form.font_family"
                placeholder="输入字体名称，多个以逗号分隔"
              />
              <span class="sd-font-preview" :style="{ fontFamily: form.font_family, fontWeight: form.font_weight }">
                AaBbCc 0123&nbsp;&nbsp;if (x) { return; }&nbsp;&nbsp;→ ≠ ≤ ≥
              </span>
            </div>

            <div class="sd-field">
              <label>字号</label>
              <div class="sd-range-row">
                <input type="range" v-model.number="form.font_size" min="10" max="24" step="1" />
                <span class="sd-range-val">{{ form.font_size }}px</span>
              </div>
            </div>

            <div class="sd-field">
              <label>字重</label>
              <select v-model.number="form.font_weight" class="sd-select">
                <option v-for="w in FONT_WEIGHTS" :key="w.value" :value="w.value">{{ w.label }}</option>
              </select>
            </div>

            <div class="sd-field">
              <label>光标样式</label>
              <div class="sd-radio-group">
                <label v-for="c in cursorStyles" :key="c.value" class="sd-radio">
                  <input type="radio" v-model="form.cursor_style" :value="c.value" />
                  <span>{{ c.label }}</span>
                </label>
              </div>
            </div>

            <div class="sd-field">
              <label>光标闪烁</label>
              <label class="sd-toggle">
                <input type="checkbox" v-model="form.cursor_blink" />
                <span class="sd-toggle-track" />
              </label>
            </div>

            <div class="sd-field">
              <label>回滚行数</label>
              <div class="sd-range-row">
                <input type="range" v-model.number="form.scrollback" min="1000" max="200000" step="1000" />
                <span class="sd-range-val">{{ form.scrollback }}</span>
              </div>
            </div>

            <div class="sd-section-title" style="margin-top:14px">本地终端</div>
            <p class="sd-desc">底部本地终端面板（Ctrl+`）的「自定义」shell。新建终端菜单内置 PowerShell 与 cmd；此处填写后会多出一个「自定义」选项（如 Git Bash），留空则不显示。</p>
            <div class="sd-field">
              <label>自定义 shell 路径</label>
              <input
                v-model="form.local_shell"
                placeholder="如 C:\Program Files\Git\bin\bash.exe，留空则不显示自定义项"
                spellcheck="false"
              />
            </div>
          </template>

          <!-- ── 界面 ── -->
          <template v-if="activeTab === 'interface'">
            <div class="sd-section-title">会话列表</div>

            <div class="sd-field">
              <label>显示「最近打开」分组</label>
              <label class="sd-toggle">
                <input type="checkbox" v-model="form.show_recent" />
                <span class="sd-toggle-track" />
              </label>
            </div>
          </template>

          <!-- ── 密钥管理 ── -->
          <template v-if="activeTab === 'keys'">
            <div class="sd-section-title">生成 SSH 密钥对</div>

            <div class="sd-field">
              <label>密钥类型</label>
              <div class="sd-radio-group">
                <label class="sd-radio">
                  <input type="radio" v-model="keyForm.type" value="ed25519" />
                  <span>Ed25519（推荐）</span>
                </label>
                <label class="sd-radio">
                  <input type="radio" v-model="keyForm.type" value="rsa4096" />
                  <span>RSA 4096</span>
                </label>
              </div>
            </div>

            <div class="sd-field">
              <label>注释（可选）</label>
              <input v-model="keyForm.comment" placeholder="your@email.com" />
            </div>

            <button class="sd-btn sd-btn-primary" :disabled="generatingKey" @click="generateKey">
              {{ generatingKey ? '生成中…' : '生成密钥对' }}
            </button>

            <div v-if="keyResult" class="sd-key-result">
              <div class="sd-key-path">
                <span class="sd-key-path-label">私钥路径：</span>
                <code>{{ keyResult.private_key_path }}</code>
              </div>
              <div class="sd-field" style="margin-top:10px;">
                <label>公钥内容（复制到服务器的 authorized_keys）</label>
                <textarea v-model="keyResult.public_key" readonly rows="3" class="sd-pubkey" />
              </div>
              <button class="sd-btn" @click="copyPubKey">复制公钥</button>
            </div>

            <div v-if="keyError" class="sd-error">{{ keyError }}</div>
          </template>

          <!-- ── 导入会话 ── -->
          <template v-if="activeTab === 'import'">
            <div class="sd-section-title">从 ~/.ssh/config 导入</div>
            <p class="sd-desc">
              解析本机 <code>~/.ssh/config</code> 中的 Host 配置块，
              将每个主机条目作为新会话导入（跳过含通配符的条目）。
            </p>
            <button class="sd-btn sd-btn-primary" :disabled="importing" @click="importSSHConfig">
              {{ importing ? '导入中…' : '开始导入' }}
            </button>
            <div v-if="importMsg" class="sd-info">{{ importMsg }}</div>
            <div v-if="importError" class="sd-error">{{ importError }}</div>
          </template>

          <!-- ── 文件管理 ── -->
          <template v-if="activeTab === 'filemanager'">
            <div class="sd-section-title">文件列表列</div>
            <p class="sd-desc">勾选控制是否显示，▲▼ 调整列顺序（文件名列始终显示）。</p>
            <div class="sd-col-list">
              <div
                v-for="(col, idx) in fmColOrder"
                :key="col"
                class="sd-col-item"
                :class="{ disabled: col === 'name' }"
              >
                <div class="sd-col-move">
                  <button class="sd-col-arrow" :disabled="idx === 0" title="上移" @click="moveFmCol(idx, -1)">▲</button>
                  <button class="sd-col-arrow" :disabled="idx === fmColOrder.length - 1" title="下移" @click="moveFmCol(idx, 1)">▼</button>
                </div>
                <label class="sd-col-check">
                  <input
                    type="checkbox"
                    :checked="isFmColChecked(col)"
                    :disabled="col === 'name'"
                    @change="toggleFmColumn(col)"
                  />
                  <span>{{ FM_COLUMN_LABELS[col] }}</span>
                </label>
              </div>
            </div>

            <div class="sd-section-title" style="margin-top:14px">默认下载目录</div>
            <p class="sd-desc">右键下载时的默认保存路径，留空则每次自动使用系统下载目录。</p>
            <div class="sd-dir-row">
              <input
                v-model="form.download_dir"
                class="sd-dir-input"
                placeholder="留空使用系统下载目录"
                spellcheck="false"
              />
              <button class="sd-btn" style="flex-shrink:0" @click="browseDownloadDirSetting">浏览…</button>
            </div>
          </template>

          <!-- ── 高亮规则 ── -->
          <template v-if="activeTab === 'highlight'">
            <div class="sd-section-title">输出高亮规则</div>
            <p class="sd-desc">对终端输出中匹配正则的文字应用颜色。服务端已着色的文字不受影响。</p>

            <div class="hl-list">
              <div class="hl-header">
                <span class="hl-col-en">启用</span>
                <span class="hl-col-name">规则名</span>
                <span class="hl-col-pattern">正则表达式</span>
                <span class="hl-col-colors">颜色</span>
                <span class="hl-col-del" />
              </div>

              <div class="hl-body">
                <div
                  v-for="(rule, idx) in hlRules"
                  :key="rule.id"
                  class="hl-row"
                >
                  <!-- 启用开关 -->
                  <label class="sd-toggle hl-col-en">
                    <input type="checkbox" v-model="rule.enabled" />
                    <span class="sd-toggle-track" />
                  </label>

                  <!-- 规则名 -->
                  <div class="hl-col-name">
                    <input
                      v-model="rule.name"
                      class="hl-input"
                      placeholder="名称"
                      :class="{ 'hl-disabled': !rule.enabled }"
                    />
                  </div>

                  <!-- 正则 -->
                  <div class="hl-col-pattern">
                    <input
                      v-model="rule.pattern"
                      class="hl-input"
                      placeholder="正则，如 \bERROR\b"
                      :class="{ 'hl-disabled': !rule.enabled, 'hl-invalid': !isValidRegex(rule.pattern) }"
                      :title="!isValidRegex(rule.pattern) ? '正则语法错误' : ''"
                    />
                  </div>

                  <!-- 前景色 + 背景色 -->
                  <div class="hl-col-colors hl-colors-wrap">
                    <label class="hl-color-label" title="文字颜色">
                      <span class="hl-swatch" :style="{ background: rule.color || 'transparent' }" />
                      <input type="color" :value="rule.color" @input="rule.color = ($event.target as HTMLInputElement).value" />
                    </label>
                    <label class="hl-color-label" title="背景颜色（可选）">
                      <span
                        class="hl-swatch hl-swatch-bg"
                        :style="rule.bg_color ? { background: rule.bg_color } : {}"
                      />
                      <input type="color" :value="rule.bg_color || '#1a1a1a'" @input="rule.bg_color = ($event.target as HTMLInputElement).value" />
                    </label>
                    <button
                      v-if="rule.bg_color"
                      class="hl-clear-bg"
                      title="清除背景色"
                      @click="rule.bg_color = undefined"
                    >✕</button>
                  </div>

                  <!-- 删除 -->
                  <button class="hl-col-del hl-del-btn" title="删除" @click="removeHlRule(idx)">🗑</button>
                </div>

                <div v-if="hlRules.length === 0" class="hl-empty">暂无规则，点击下方添加</div>
              </div>
            </div>

            <div class="hl-actions">
              <button class="sd-btn" @click="addHlRule">＋ 添加规则</button>
              <button class="sd-btn" @click="resetHlRules">恢复默认</button>
              <button class="sd-btn" @click="showHlPreview = !showHlPreview">{{ showHlPreview ? '隐藏预览' : '预览效果' }}</button>
            </div>

            <!-- 预览面板 -->
            <div v-if="showHlPreview" class="hl-preview">
              <div class="hl-preview-title">预览（应用当前规则）：</div>
              <pre class="hl-preview-content" v-html="hlPreviewHtml"></pre>
              <div class="hl-preview-tip">⚠ 此预览使用当前编辑中的规则；若要在终端生效需点击"保存设置"</div>
            </div>
          </template>

          <!-- ── 关于 ── -->
          <template v-if="activeTab === 'about'">
            <div class="sd-about">
              <img :src="logoUrl" class="sd-about-logo" alt="Conch" />
              <div class="sd-about-name">Conch</div>
              <div class="sd-about-ver">v{{ appVersion }}</div>
              <div class="sd-about-desc">Every shell, in one shell · 基于 Wails + Vue 3 的跨平台 SSH 客户端</div>
              <div class="sd-about-stack">
                <span class="sd-tag">Go</span>
                <span class="sd-tag">Wails v2</span>
                <span class="sd-tag">Vue 3</span>
                <span class="sd-tag">xterm.js</span>
              </div>
              <div class="sd-about-meta">
                <div class="sd-meta-row">
                  <span class="sd-meta-label">版本</span>
                  <span class="sd-meta-value">v{{ appVersion }}</span>
                </div>
                <div class="sd-meta-row">
                  <span class="sd-meta-label">作者</span>
                  <span class="sd-meta-value">xt_zsh</span>
                </div>
                <div class="sd-meta-row">
                  <span class="sd-meta-label">配置目录</span>
                  <span class="sd-meta-value sd-meta-mono">~/.config/conch/</span>
                </div>
                <div class="sd-meta-row">
                  <span class="sd-meta-label">会话文件</span>
                  <span class="sd-meta-value sd-meta-mono">sessions.json</span>
                </div>
                <div class="sd-meta-row">
                  <span class="sd-meta-label">设置文件</span>
                  <span class="sd-meta-value sd-meta-mono">settings.json</span>
                </div>
              </div>
            </div>
          </template>

        </div>
      </div>

      <!-- 底部：自动保存状态 + 关闭 -->
      <div class="sd-footer">
        <span class="sd-autosave-hint">
          <template v-if="saveStatus === 'saving'">💾 保存中…</template>
          <template v-else-if="saveStatus === 'saved'">✓ 已自动保存</template>
          <template v-else>修改后自动保存</template>
        </span>
        <button class="sd-btn" @click="$emit('close')">关闭</button>
      </div>

    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted, onUnmounted } from 'vue'
import { useSettingsStore, type Settings, type HighlightRule, DEFAULT_HIGHLIGHT_RULES, FM_ALL_COLUMNS, FM_COLUMN_LABELS, type FMColumn } from '../stores/settings'
import { useDraggable } from '../composables/useDraggable'
import logoUrl from '../assets/images/logo.png'

const { boxStyle, startDrag } = useDraggable()
import { GenerateKey, ImportSSHConfig, GetVersion, OpenDirectoryDialog } from '../../wailsjs/go/main/App'

const emit = defineEmits<{ close: []; saved: [] }>()

const settingsStore = useSettingsStore()
const saving     = ref(false)
const saveStatus = ref<'idle' | 'saving' | 'saved'>('idle')
const appVersion = ref('…')
const activeTab  = ref('terminal')

// ── 自动保存（300ms 防抖） ────────────────────────────────────────────────────
let _autoSaveTimer: ReturnType<typeof setTimeout> | null = null
let _initDone = false   // 防止 onMounted 加载完初始值就立即触发保存

function scheduleAutoSave() {
  if (!_initDone) return
  if (_autoSaveTimer) clearTimeout(_autoSaveTimer)
  _autoSaveTimer = setTimeout(async () => {
    _autoSaveTimer = null
    saving.value     = true
    saveStatus.value = 'saving'
    try {
      await settingsStore.save({ ...form, highlight_rules: hlRules.value })
      saveStatus.value = 'saved'
      emit('saved')
      setTimeout(() => { saveStatus.value = 'idle' }, 2000)
    } finally {
      saving.value = false
    }
  }, 300)
}

onUnmounted(() => { if (_autoSaveTimer) clearTimeout(_autoSaveTimer) })

const tabs = [
  { id: 'terminal',    label: '终端外观' },
  { id: 'interface',   label: '界面' },
  { id: 'highlight',   label: '高亮规则' },
  { id: 'keys',        label: 'SSH 密钥' },
  { id: 'import',      label: '导入会话' },
  { id: 'filemanager', label: '文件管理' },
  { id: 'about',       label: '关于' },
]

// 文件列表列顺序（含隐藏列）：可见列在前、按 fm_columns 顺序，隐藏列补在后面。
// form.fm_columns 始终 = fmColOrder 中「可见」列，按此顺序保存。
const fmColOrder = ref<FMColumn[]>([])

function initFmColOrder() {
  const inCols = (form.fm_columns as FMColumn[]).filter(c => (FM_ALL_COLUMNS as readonly string[]).includes(c))
  const rest = (FM_ALL_COLUMNS as readonly FMColumn[]).filter(c => !inCols.includes(c))
  fmColOrder.value = [...inCols, ...rest]
}

function isFmColChecked(col: FMColumn) {
  return form.fm_columns.includes(col)
}

// 按当前 fmColOrder 顺序、保留可见集，重建 form.fm_columns
function rebuildFmColumns(visible: Set<FMColumn>) {
  form.fm_columns = fmColOrder.value.filter(c => visible.has(c))
}

function toggleFmColumn(col: FMColumn) {
  if (col === 'name') return
  const visible = new Set(form.fm_columns as FMColumn[])
  if (visible.has(col)) visible.delete(col)
  else visible.add(col)
  rebuildFmColumns(visible)
}

function moveFmCol(idx: number, dir: -1 | 1) {
  const to = idx + dir
  if (to < 0 || to >= fmColOrder.value.length) return
  const arr = [...fmColOrder.value]
  ;[arr[idx], arr[to]] = [arr[to], arr[idx]]
  fmColOrder.value = arr
  rebuildFmColumns(new Set(form.fm_columns as FMColumn[]))
}

// ── Color scheme ─────────────────────────────────────────────
const COLOR_SCHEME_OPTIONS = [
  { value: 'conch',           label: 'Conch（默认）' },
  { value: 'github-dark',     label: 'GitHub Dark' },
  { value: 'dracula',         label: 'Dracula' },
  { value: 'one-dark',        label: 'One Dark' },
  { value: 'monokai',         label: 'Monokai' },
  { value: 'solarized-dark',  label: 'Solarized Dark' },
  { value: 'nord',            label: 'Nord' },
  { value: 'catppuccin',      label: 'Catppuccin Mocha' },
  { value: 'material',        label: 'Material Dark' },
  { value: 'solarized-light', label: 'Solarized Light' },
  { value: 'dark-charcoal',   label: 'Dark Charcoal（#1c1c1c）' },
  { value: 'carbon',          label: 'Carbon（碳黑 #0c0c0c）' },
  { value: 'pure-black',      label: 'Pure Black（纯黑 #000000）' },
]

const SCHEME_PALETTES: Record<string, string[]> = {
  'conch':           ['#0b0e13','#ef6d6d','#46cf8b','#ecb44a','#34c8d4','#a98bf5','#5aa2fb','#d7dee7'],
  'github-dark':     ['#0d1117','#ff7b72','#3fb950','#d29922','#58a6ff','#bc8cff','#39c5cf','#e6edf3'],
  'dracula':         ['#282a36','#ff5555','#50fa7b','#f1fa8c','#bd93f9','#ff79c6','#8be9fd','#f8f8f2'],
  'one-dark':        ['#282c34','#e06c75','#98c379','#e5c07b','#61afef','#c678dd','#56b6c2','#abb2bf'],
  'monokai':         ['#272822','#f92672','#a6e22e','#f4bf75','#66d9e8','#ae81ff','#a1efe4','#f8f8f2'],
  'solarized-dark':  ['#002b36','#dc322f','#859900','#b58900','#268bd2','#d33682','#2aa198','#839496'],
  'nord':            ['#2e3440','#bf616a','#a3be8c','#ebcb8b','#81a1c1','#b48ead','#88c0d0','#d8dee9'],
  'catppuccin':      ['#1e1e2e','#f38ba8','#a6e3a1','#f9e2af','#89b4fa','#f5c2e7','#94e2d5','#cdd6f4'],
  'material':        ['#263238','#ff5370','#c3e88d','#ffcb6b','#82aaff','#c792ea','#89ddff','#eeffff'],
  'solarized-light': ['#fdf6e3','#dc322f','#859900','#b58900','#268bd2','#d33682','#2aa198','#657b83'],
  'dark-charcoal':   ['#1c1c1c','#f44747','#6a9955','#dcdcaa','#569cd6','#c678dd','#4ec9b0','#d4d4d4'],
  'carbon':          ['#0c0c0c','#c50f1f','#13a10e','#c19c00','#0037da','#881798','#3a96dd','#f2f2f2'],
  'pure-black':      ['#000000','#ff5555','#55ff55','#ffff55','#5555ff','#ff55ff','#55ffff','#ffffff'],
}

const currentSchemePreview = computed(() =>
  SCHEME_PALETTES[form.color_scheme] ?? SCHEME_PALETTES['github-dark']
)

// ── Font presets ──────────────────────────────────────────────
const FONT_PRESETS = [
  { value: 'Cascadia Code',      label: 'Cascadia Code' },
  { value: 'Cascadia Mono',      label: 'Cascadia Mono' },
  { value: 'JetBrains Mono',     label: 'JetBrains Mono' },
  { value: 'Fira Code',          label: 'Fira Code' },
  { value: 'Roboto Mono',        label: 'Roboto Mono' },
  { value: 'Source Code Pro',    label: 'Source Code Pro' },
  { value: 'Hack',               label: 'Hack' },
  { value: 'Inconsolata',        label: 'Inconsolata' },
  { value: 'Consolas',           label: 'Consolas（Windows 内置）' },
  { value: 'Courier New',        label: 'Courier New（通用）' },
  { value: 'Monaco',             label: 'Monaco（macOS）' },
  { value: 'Menlo',              label: 'Menlo（macOS）' },
  { value: 'Ubuntu Mono',        label: 'Ubuntu Mono（Linux）' },
  { value: 'DejaVu Sans Mono',   label: 'DejaVu Sans Mono（Linux）' },
  { value: 'monospace',          label: '系统默认等宽字体' },
]

const FONT_WEIGHTS = [
  { value: 100, label: '100 · Thin' },
  { value: 200, label: '200 · ExtraLight' },
  { value: 300, label: '300 · Light' },
  { value: 400, label: '400 · Regular' },
  { value: 500, label: '500 · Medium' },
  { value: 600, label: '600 · SemiBold' },
  { value: 700, label: '700 · Bold' },
  { value: 800, label: '800 · ExtraBold' },
  { value: 900, label: '900 · Black' },
]

const fontSelectValue = computed({
  get: () => FONT_PRESETS.some(f => f.value === form.font_family) ? form.font_family : '__custom__',
  set: (val: string) => { if (val !== '__custom__') form.font_family = val },
})

const cursorStyles = [
  { value: 'block',     label: '方块' },
  { value: 'bar',       label: '竖线' },
  { value: 'underline', label: '下划线' },
]

// Copy current settings into local form
const form = reactive<Settings>({ ...settingsStore.settings })

// ── 高亮规则（单独管理，避免 reactive 深层引用问题） ────────────────────────
const hlRules = ref<HighlightRule[]>([])

function isValidRegex(pattern: string): boolean {
  if (!pattern) return true
  try { new RegExp(pattern); return true } catch { return false }
}

function addHlRule() {
  hlRules.value.push({
    id:      Date.now().toString(),
    name:    '新规则',
    pattern: '',
    color:   '#ffffff',
    enabled: true,
  })
}

function removeHlRule(idx: number) {
  hlRules.value.splice(idx, 1)
}

function resetHlRules() {
  hlRules.value = JSON.parse(JSON.stringify(DEFAULT_HIGHLIGHT_RULES))
}

// ── 高亮预览 ────────────────────────────────────────────────────────────────
const showHlPreview = ref(false)

const HL_PREVIEW_SAMPLE = `2024-05-07 10:32:18 [INFO] Server started on 192.168.1.100:8080
2024-05-07 10:32:19 [DEBUG] Loading config from /etc/myapp/config.yaml
2024-05-07 10:32:20 [WARN] deprecated option: use new_format=true instead
GET /api/users/123 HTTP/1.1 → 200 OK in 45ms
POST /api/login → 401 Unauthorized
DELETE /api/items/999 → 500 Internal Server Error
[ERROR] connection failed: timeout after 30 seconds
[FATAL] PANIC: out of memory at $HOME/server/main.go
status=true count=42 ratio=0.85 size=128MB null_field=null`

function hexToRgbStr(hex: string): string {
  const m = /^#?([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(hex)
  return m ? `${parseInt(m[1],16)},${parseInt(m[2],16)},${parseInt(m[3],16)}` : '255,255,255'
}

// 与 Terminal.vue 保持一致的高亮引擎（单遍扫描，避免规则间冲突）
function applyRulesToText(text: string, rules: HighlightRule[]): string {
  interface Hit { start: number; end: number; color: string; bg: string }
  const hits: Hit[] = []
  for (const r of rules) {
    if (!r.enabled || !r.pattern) continue
    try {
      const re = new RegExp(r.pattern, 'g')
      let m: RegExpExecArray | null
      while ((m = re.exec(text)) !== null) {
        const s = m.index, e = m.index + m[0].length
        if (!hits.some(h => s < h.end && e > h.start)) {
          hits.push({ start: s, end: e, color: r.color, bg: r.bg_color || '' })
        }
      }
    } catch {}
  }
  if (hits.length === 0) return escapeHtml(text)
  hits.sort((a, b) => a.start - b.start)
  let out = '', pos = 0
  for (const h of hits) {
    out += escapeHtml(text.slice(pos, h.start))
    const style = `color: rgb(${hexToRgbStr(h.color)});` + (h.bg ? `background: rgb(${hexToRgbStr(h.bg)});` : '')
    out += `<span style="${style}">${escapeHtml(text.slice(h.start, h.end))}</span>`
    pos = h.end
  }
  return out + escapeHtml(text.slice(pos))
}

function escapeHtml(s: string): string {
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;')
}

const hlPreviewHtml = computed(() => applyRulesToText(HL_PREVIEW_SAMPLE, hlRules.value))

onMounted(() => {
  Object.assign(form, settingsStore.settings)
  initFmColOrder()
  hlRules.value = JSON.parse(JSON.stringify(
    settingsStore.settings.highlight_rules?.length
      ? settingsStore.settings.highlight_rules
      : DEFAULT_HIGHLIGHT_RULES
  ))
  GetVersion().then(v => { appVersion.value = v }).catch(() => {})
  // 初始加载完成后才开始监听改动，防止误触发自动保存
  setTimeout(() => { _initDone = true }, 0)
})

// 监听表单任何字段改动 → 触发自动保存
watch(form, scheduleAutoSave, { deep: true })
watch(hlRules, scheduleAutoSave, { deep: true })

// 手动保存（兜底，快捷键或意外情况用）
async function onSave() {
  if (_autoSaveTimer) { clearTimeout(_autoSaveTimer); _autoSaveTimer = null }
  saving.value = true
  saveStatus.value = 'saving'
  try {
    await settingsStore.save({ ...form, highlight_rules: hlRules.value })
    saveStatus.value = 'saved'
    emit('saved')
    emit('close')
  } finally {
    saving.value = false
  }
}

// ── Key generation ──────────────────────────────────────────

const keyForm = reactive({ type: 'ed25519', comment: '' })
const generatingKey = ref(false)
const keyResult = ref<{ private_key_path: string; public_key: string } | null>(null)
const keyError = ref('')

async function generateKey() {
  keyError.value = ''
  keyResult.value = null
  generatingKey.value = true
  try {
    const r = await GenerateKey(keyForm.type, keyForm.comment)
    keyResult.value = r
  } catch (e: any) {
    keyError.value = String(e)
  } finally {
    generatingKey.value = false
  }
}

function copyPubKey() {
  if (keyResult.value) {
    navigator.clipboard.writeText(keyResult.value.public_key)
  }
}

// ── Download dir (settings page) ────────────────────────────
async function browseDownloadDirSetting() {
  const dir = await OpenDirectoryDialog()
  if (dir) form.download_dir = dir
}

// ── SSH config import ────────────────────────────────────────

const importing = ref(false)
const importMsg = ref('')
const importError = ref('')

async function importSSHConfig() {
  importMsg.value = ''
  importError.value = ''
  importing.value = true
  try {
    const count = await ImportSSHConfig()
    importMsg.value = `成功导入 ${count} 个会话`
    if (count > 0) emit('saved')
  } catch (e: any) {
    importError.value = String(e)
  } finally {
    importing.value = false
  }
}
</script>

<style scoped>
.sd-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.65);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 200;
  backdrop-filter: blur(2px);
}

.sd-dialog {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 10px;
  width: 580px;
  height: 540px;
  max-height: 88vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 24px 48px rgba(0,0,0,0.6);
}

.sd-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px 10px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.sd-title {
  font-size: 14px;
  font-weight: 600;
}

.sd-icon-btn {
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 12px;
  padding: 2px 5px;
  border-radius: 3px;
}
.sd-icon-btn:hover { color: var(--text-primary); background: rgba(255,255,255,0.06); }

/* ── Layout ── */
.sd-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

.sd-nav {
  width: 110px;
  flex-shrink: 0;
  padding: 10px 0;
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.sd-nav-item {
  background: transparent;
  border: none;
  border-radius: 0;
  text-align: left;
  padding: 8px 14px;
  font-size: 12px;
  color: var(--text-muted);
  cursor: pointer;
  transition: color 0.15s, background 0.15s;
}
.sd-nav-item:hover { color: var(--text-primary); background: rgba(255,255,255,0.04); }
.sd-nav-item.active { color: var(--accent); background: rgba(88,166,255,0.08); }

.sd-content {
  flex: 1;
  padding: 16px 20px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

/* ── Form ── */
.sd-section-title {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.4px;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--border);
  margin-bottom: 2px;
}

.sd-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.sd-field label {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
}

.sd-radio-group {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}

.sd-radio {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-primary);
  cursor: pointer;
}

.sd-radio input { width: auto; cursor: pointer; }

.sd-range-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.sd-range-row input[type="range"] {
  flex: 1;
  padding: 0;
  border: none;
  background: transparent;
  cursor: pointer;
}

.sd-range-val {
  font-size: 12px;
  color: var(--text-muted);
  min-width: 44px;
  text-align: right;
}

/* Toggle switch */
.sd-toggle {
  display: flex;
  align-items: center;
  cursor: pointer;
  width: fit-content;
}

.sd-toggle input { display: none; }

.sd-toggle-track {
  display: inline-block;
  width: 36px;
  height: 20px;
  background: var(--border);
  border-radius: 10px;
  position: relative;
  transition: background 0.2s;
}

.sd-toggle-track::after {
  content: '';
  position: absolute;
  top: 3px;
  left: 3px;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: #fff;
  transition: transform 0.2s;
}

.sd-toggle input:checked + .sd-toggle-track { background: var(--accent); }
.sd-toggle input:checked + .sd-toggle-track::after { transform: translateX(16px); }

/* ── Buttons ── */
.sd-btn {
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 5px;
  padding: 5px 16px;
  font-size: 12px;
  color: var(--text-primary);
  cursor: pointer;
  align-self: flex-start;
  transition: background 0.15s;
}
.sd-btn:hover:not(:disabled) { background: rgba(255,255,255,0.06); }
.sd-btn:disabled { opacity: 0.4; cursor: not-allowed; }

.sd-btn-primary {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}
.sd-btn-primary:hover:not(:disabled) { filter: brightness(1.1); }

/* ── Key result ── */
.sd-key-result {
  background: rgba(88,166,255,0.06);
  border: 1px solid rgba(88,166,255,0.2);
  border-radius: 7px;
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.sd-key-path { font-size: 12px; }
.sd-key-path-label { color: var(--text-muted); }
.sd-key-path code { font-family: monospace; color: var(--text-primary); }

.sd-pubkey {
  font-family: monospace;
  font-size: 11px;
  resize: none;
  background: var(--bg-primary);
  color: var(--text-primary);
  padding: 8px;
}

/* ── Column selector ── */
.sd-col-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.sd-col-item {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12px;
  color: var(--text-primary);
}

.sd-col-item.disabled { opacity: 0.5; cursor: not-allowed; }

/* 覆盖全局 input { width: 100% } */
.sd-col-item input[type="checkbox"] {
  width: auto;
  flex-shrink: 0;
  padding: 0;
  border: none;
  background: transparent;
  cursor: pointer;
}

.sd-col-item.disabled input[type="checkbox"] { cursor: not-allowed; }

.sd-col-item span { white-space: nowrap; }

/* 列顺序上下按钮 */
.sd-col-move { display: flex; flex-direction: column; gap: 1px; flex-shrink: 0; }
.sd-col-arrow {
  width: 20px; height: 12px;
  display: flex; align-items: center; justify-content: center;
  background: transparent; border: 1px solid var(--border); border-radius: 3px;
  color: var(--text-muted); font-size: 8px; line-height: 1; padding: 0; cursor: pointer;
  transition: color 0.15s, background 0.15s, border-color 0.15s;
}
.sd-col-arrow:hover:not(:disabled) { color: var(--text-primary); border-color: var(--accent); background: rgba(88,166,255,0.08); }
.sd-col-arrow:disabled { opacity: 0.3; cursor: not-allowed; }
.sd-col-check { display: flex; align-items: center; gap: 8px; cursor: pointer; min-width: 0; }
.sd-col-item.disabled .sd-col-check { cursor: not-allowed; }

/* ── Color scheme palette ── */
.sd-palette {
  display: flex;
  gap: 5px;
  align-items: center;
  flex-wrap: wrap;
}

.sd-swatch {
  width: 20px;
  height: 20px;
  border-radius: 4px;
  border: 1px solid rgba(255,255,255,0.12);
  flex-shrink: 0;
  cursor: default;
}

/* ── Font select & preview ── */
.sd-select {
  width: 100%;
  padding: 5px 8px;
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text-primary);
  font-size: 12px;
  cursor: pointer;
  outline: none;
  appearance: auto;
}
.sd-select:focus { border-color: var(--accent); }

.sd-font-preview {
  display: block;
  padding: 7px 10px;
  background: var(--bg-primary);
  border: 1px dashed var(--border);
  border-radius: 5px;
  font-size: 13px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  letter-spacing: 0.02em;
}

/* ── Messages ── */
.sd-desc {
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.6;
}

.sd-info {
  background: rgba(63,185,80,0.1);
  border: 1px solid rgba(63,185,80,0.3);
  border-radius: 6px;
  padding: 8px 12px;
  font-size: 12px;
  color: var(--success);
}

.sd-error {
  background: rgba(248,81,73,0.1);
  border: 1px solid rgba(248,81,73,0.3);
  border-radius: 6px;
  padding: 8px 12px;
  font-size: 12px;
  color: var(--danger);
}

/* ── Footer ── */
.sd-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  padding: 10px 18px;
  border-top: 1px solid var(--border);
  flex-shrink: 0;
}

.sd-autosave-hint {
  flex: 1;
  font-size: 11px;
  color: var(--text-muted);
  opacity: 0.7;
}

/* ── 关于页 ── */
.sd-about {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 24px 0 16px;
}

.sd-about-logo {
  width: 72px;
  height: 72px;
  border-radius: 16px;
  display: block;
  filter: drop-shadow(0 4px 14px rgba(59,130,246,0.45));
}

.sd-about-name {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: 1px;
}

.sd-about-ver {
  font-size: 13px;
  color: var(--accent);
  font-weight: 600;
  background: rgba(88,166,255,0.1);
  border: 1px solid rgba(88,166,255,0.25);
  border-radius: 20px;
  padding: 2px 14px;
}

.sd-about-desc {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 2px;
}

.sd-about-stack {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  justify-content: center;
  margin: 4px 0 8px;
}

.sd-tag {
  font-size: 11px;
  color: var(--text-muted);
  background: rgba(255,255,255,0.06);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 2px 8px;
}

.sd-about-meta {
  width: 100%;
  max-width: 320px;
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
  margin-top: 4px;
}

.sd-meta-row {
  display: flex;
  align-items: center;
  padding: 7px 14px;
  border-bottom: 1px solid rgba(255,255,255,0.04);
  gap: 12px;
}

.sd-meta-row:last-child { border-bottom: none; }

.sd-meta-label {
  width: 70px;
  font-size: 11px;
  color: var(--text-muted);
  flex-shrink: 0;
}

.sd-meta-value {
  font-size: 12px;
  color: var(--text-primary);
}

.sd-meta-mono {
  font-family: 'Cascadia Code', Consolas, monospace;
  font-size: 11px;
  color: var(--text-muted);
}

/* ── 高亮规则 ── */
.hl-list {
  border: 1px solid var(--border);
  border-radius: 7px;
  overflow: hidden;
  font-size: 12px;
}

.hl-body {
  max-height: 260px;
  overflow-y: auto;
}

.hl-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 8px;
  background: rgba(255,255,255,0.03);
  border-bottom: 1px solid var(--border);
  font-size: 11px;
  color: var(--text-muted);
  font-weight: 600;
}

.hl-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 8px;
  border-bottom: 1px solid rgba(255,255,255,0.04);
  transition: background 0.1s;
}
.hl-row:last-child { border-bottom: none; }
.hl-row:hover { background: rgba(255,255,255,0.02); }

/* Column widths — these are on wrapper divs/spans, NOT on the inputs directly */
.hl-col-en      { width: 36px; flex-shrink: 0; }
.hl-col-name    { width: 72px; flex-shrink: 0; display: flex; align-items: center; }
.hl-col-pattern { flex: 1; min-width: 0; display: flex; align-items: center; }
.hl-col-colors  { width: 80px; flex-shrink: 0; }
.hl-col-del     { width: 24px; flex-shrink: 0; }

.hl-input {
  width: 100%;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 4px;
  color: var(--text-primary);
  font-size: 12px;
  padding: 3px 6px;
  outline: none;
  transition: border-color 0.15s;
}
.hl-input:focus  { border-color: var(--accent); background: var(--bg-primary); }
.hl-input:hover  { border-color: var(--border); }
.hl-disabled     { opacity: 0.4; }
.hl-invalid      { border-color: var(--danger) !important; }

/* Color swatches */
.hl-colors-wrap {
  display: flex;
  align-items: center;
  gap: 4px;
}

.hl-color-label {
  position: relative;
  display: inline-flex;
  cursor: pointer;
}

.hl-color-label input[type="color"] {
  position: absolute;
  inset: 0;
  opacity: 0;
  cursor: pointer;
  width: 100%;
  height: 100%;
  padding: 0;
  border: none;
}

.hl-swatch {
  display: block;
  width: 20px;
  height: 20px;
  border-radius: 4px;
  border: 1px solid rgba(255,255,255,0.15);
  flex-shrink: 0;
}

.hl-swatch-bg {
  background: repeating-conic-gradient(rgba(255,255,255,0.08) 0% 25%, transparent 0% 50%) 0 0 / 8px 8px;
}

.hl-clear-bg {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 9px;
  cursor: pointer;
  padding: 1px 2px;
  border-radius: 3px;
  line-height: 1;
}
.hl-clear-bg:hover { color: var(--danger); }

.hl-del-btn {
  background: transparent;
  border: none;
  cursor: pointer;
  font-size: 13px;
  opacity: 0.4;
  padding: 1px 2px;
  border-radius: 3px;
  transition: opacity 0.15s;
}
.hl-del-btn:hover { opacity: 1; }

.hl-empty {
  padding: 16px;
  text-align: center;
  color: var(--text-muted);
  font-size: 12px;
}

.hl-actions {
  display: flex;
  gap: 8px;
  margin-top: 2px;
}

.hl-preview {
  margin-top: 6px;
  border: 1px solid var(--border);
  border-radius: 7px;
  overflow: hidden;
}

.hl-preview-title {
  padding: 6px 10px;
  font-size: 11px;
  color: var(--text-muted);
  background: rgba(255,255,255,0.03);
  border-bottom: 1px solid var(--border);
}

.hl-preview-content {
  margin: 0;
  padding: 10px;
  font-family: 'Cascadia Code', Consolas, monospace;
  font-size: 11px;
  line-height: 1.5;
  background: #0d1117;
  color: #e6edf3;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 200px;
  overflow-y: auto;
}

.hl-preview-tip {
  padding: 5px 10px;
  font-size: 10px;
  color: var(--text-muted);
  background: rgba(248,165,73,0.05);
  border-top: 1px solid var(--border);
}

/* ── Download dir row ── */
.sd-dir-row {
  display: flex;
  gap: 6px;
  align-items: center;
}

.sd-dir-input {
  flex: 1;
  min-width: 0;
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text-primary);
  font-size: 12px;
  padding: 5px 8px;
  outline: none;
  font-family: 'Cascadia Code', Consolas, monospace;
  transition: border-color 0.15s;
}
.sd-dir-input:focus { border-color: var(--accent); }
.sd-dir-input::placeholder { color: var(--text-muted); opacity: 0.6; }
</style>
