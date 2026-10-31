<template>
  <Teleport to="body">
    <div v-if="visible" class="lfp-mask">
      <div class="lfp-dialog" :style="boxStyle" style="--wails-draggable:no-drag">

        <!-- 标题栏 -->
        <div class="lfp-titlebar" @mousedown.prevent="startDrag">
          <span>选择上传项目</span>
          <button class="lfp-close" @click="cancel">✕</button>
        </div>

        <!-- Look in 导航行 -->
        <div class="lfp-lookin">
          <span class="lfp-lookin-label">Look in:</span>
          <input
            v-model="pathInput"
            class="lfp-path-input"
            spellcheck="false"
            @keydown.enter.prevent="commitPath"
            @keydown.esc.prevent="pathInput = currentPath"
          />
          <button class="lfp-nav-btn" :disabled="!canBack" title="后退 (Alt+←)" @click="goBack">◀</button>
          <button class="lfp-nav-btn" :disabled="!canFwd"  title="前进 (Alt+→)" @click="goFwd">▶</button>
          <button class="lfp-nav-btn" title="上级目录" @click="goUp">▲</button>
        </div>

        <!-- 主体：侧边栏 + 文件列表 -->
        <div class="lfp-body">

          <!-- 左侧快捷访问 -->
          <div class="lfp-sidebar">
            <div
              class="lfp-sidebar-item"
              :class="{ active: currentPath === homePath }"
              @click="navigateTo(homePath)"
            >🏠 主文件夹</div>
            <template v-if="quickDirs.length">
              <div class="lfp-sidebar-sep" />
              <div
                v-for="qd in quickDirs"
                :key="qd.path"
                class="lfp-sidebar-item"
                :class="{ active: currentPath === qd.path }"
                :title="qd.path"
                @click="navigateTo(qd.path)"
              >{{ QUICK_DIR_ICONS[qd.name] ?? '📁' }} {{ qd.name }}</div>
            </template>
            <div class="lfp-sidebar-sep" />
            <div
              v-for="root in roots"
              :key="root"
              class="lfp-sidebar-item"
              :class="{ active: currentPath === root }"
              @click="navigateTo(root)"
            >💾 {{ root }}</div>
          </div>

          <!-- 文件列表 -->
          <div
            ref="listRef"
            class="lfp-list"
            tabindex="0"
            @click.self="clearSelection"
            @keydown="onKeydown"
          >
            <div v-if="loading" class="lfp-status">加载中…</div>
            <div v-else-if="listError" class="lfp-status lfp-error">{{ listError }}</div>
            <table v-else class="lfp-table">
              <colgroup>
                <col style="width:auto" />
                <col style="width:72px" />
                <col style="width:72px" />
                <col style="width:130px" />
              </colgroup>
              <thead>
                <tr>
                  <th class="lfp-th-sort" @click="toggleSort('name')">
                    名称<span class="lfp-sort-arrow">{{ sortBy === 'name' ? (sortDir === 'asc' ? ' ↑' : ' ↓') : '' }}</span>
                  </th>
                  <th class="lfp-th-sort" @click="toggleSort('size')">
                    大小<span class="lfp-sort-arrow">{{ sortBy === 'size' ? (sortDir === 'asc' ? ' ↑' : ' ↓') : '' }}</span>
                  </th>
                  <th class="lfp-th-sort" @click="toggleSort('type')">
                    类型<span class="lfp-sort-arrow">{{ sortBy === 'type' ? (sortDir === 'asc' ? ' ↑' : ' ↓') : '' }}</span>
                  </th>
                  <th class="lfp-th-sort" @click="toggleSort('mod_time')">
                    修改时间<span class="lfp-sort-arrow">{{ sortBy === 'mod_time' ? (sortDir === 'asc' ? ' ↑' : ' ↓') : '' }}</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(f, idx) in sortedFiles"
                  :key="f.path"
                  :data-idx="idx"
                  :class="{
                    selected: selectedPaths.has(f.path),
                    focused:  focusedIdx === idx,
                  }"
                  @click="onRowClick(f, idx, $event)"
                  @dblclick="onDblClick(f)"
                  @mouseenter="focusedIdx = idx"
                >
                  <td class="lfp-col-name">
                    <span class="lfp-file-icon">{{ f.is_dir ? '📁' : fileEmoji(f.name) }}</span>
                    <span :title="f.name">{{ f.name }}</span>
                  </td>
                  <td class="lfp-col-num">{{ f.is_dir ? '' : fmtSize(f.size) }}</td>
                  <td class="lfp-col-num">{{ f.is_dir ? '文件夹' : extLabel(f.name) }}</td>
                  <td class="lfp-col-num">{{ fmtTime(f.mod_time) }}</td>
                </tr>
                <tr v-if="!loading && sortedFiles.length === 0">
                  <td colspan="4" class="lfp-empty">{{
                    (showMode === 'all' && fileTypeFilter === 0)
                      ? '（空目录）'
                      : showMode === 'dirs'
                        ? '（无子目录）'
                        : showMode === 'files'
                          ? '（无文件）'
                          : '（无匹配项）'
                  }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- 底部：选择框 + 类型过滤 + 按钮 -->
        <div class="lfp-footer">
          <div class="lfp-sel-row">
            <span class="lfp-sel-label">{{ selLabelText }}：</span>
            <input
              v-model="selInput"
              class="lfp-sel-input"
              spellcheck="false"
              @keydown.enter.prevent="onSelEnter"
            />
          </div>
          <div class="lfp-type-row">
            <span class="lfp-sel-label">文件类型：</span>
            <select v-model="fileTypeFilter" class="lfp-type-select" :disabled="showMode === 'dirs'">
              <option v-for="(ft, i) in FILE_TYPE_FILTERS" :key="i" :value="i">{{ ft.label }}</option>
            </select>
            <div class="lfp-show-toggle">
              <button :class="['lfp-tog', { active: showMode === 'all' }]"   title="显示全部" @click="showMode = 'all'">全部</button>
              <button :class="['lfp-tog', { active: showMode === 'files' }]" title="仅显示文件" @click="showMode = 'files'">文件</button>
              <button :class="['lfp-tog', { active: showMode === 'dirs' }]"  title="仅显示目录" @click="showMode = 'dirs'">目录</button>
            </div>
          </div>
          <div class="lfp-btn-row">
            <button class="lfp-btn lfp-btn-primary" :disabled="!canConfirm" @click="confirm">确定</button>
            <button class="lfp-btn" @click="cancel">取消</button>
          </div>
        </div>

      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { LocalList, LocalHomePath, LocalListRoots, LocalQuickDirs } from '../../wailsjs/go/main/App'
import { useDraggable } from '../composables/useDraggable'

// ─── 拖拽 ─────────────────────────────────────────────────────────────────────

const { boxStyle, startDrag } = useDraggable()

// ─── 对外状态 ─────────────────────────────────────────────────────────────────

const visible = ref(false)
let resolveRef: ((v: string[] | null) => void) | null = null

/** 打开选择器，返回 Promise<string[] | null>（null = 取消） */
function show(): Promise<string[] | null> {
  resetState()
  visible.value = true
  return new Promise(res => { resolveRef = res })
}

function confirm() {
  if (!canConfirm.value) return
  const paths = [...selectedPaths.value]
  visible.value = false
  resolveRef?.(paths)
  resolveRef = null
}

function cancel() {
  visible.value = false
  resolveRef?.(null)
  resolveRef = null
}

defineExpose({ show })

// ─── 文件系统状态 ─────────────────────────────────────────────────────────────

interface LocalFile {
  name: string
  size: number
  is_dir: boolean
  path: string
  mod_time: string
}

interface QuickDir { name: string; path: string }

const QUICK_DIR_ICONS: Record<string, string> = {
  '桌面': '🖥️', '文档': '📝', '下载': '📥',
  '图片': '🖼️', '音乐': '🎵', '视频': '🎬',
}

// ─── 排序状态 ─────────────────────────────────────────────────────────────────

type SortKey = 'name' | 'size' | 'type' | 'mod_time'
type SortDir = 'asc' | 'desc'

const sortBy  = ref<SortKey>('mod_time')
const sortDir = ref<SortDir>('desc')

function toggleSort(key: SortKey) {
  if (sortBy.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortBy.value  = key
    sortDir.value = 'asc'
  }
}

// ─── 文件类型过滤 ─────────────────────────────────────────────────────────────

interface TypeFilter { label: string; exts: string[] | null }

const FILE_TYPE_FILTERS: TypeFilter[] = [
  { label: '所有文件',    exts: null },
  { label: '图片',        exts: ['jpg','jpeg','png','gif','webp','bmp','svg','ico','tiff','heic','avif'] },
  { label: '文档',        exts: ['pdf','doc','docx','xls','xlsx','ppt','pptx','txt','md','rtf','odt','ods','odp','csv'] },
  { label: '压缩包',      exts: ['zip','tar','gz','bz2','xz','rar','7z','tgz','zst','lz4'] },
  { label: '代码文件',    exts: ['js','ts','vue','jsx','tsx','py','go','rs','java','c','cpp','h','hpp','cs','php','rb','sh','bat','ps1','json','yaml','yml','toml','xml','html','css','scss','sass','less','sql'] },
  { label: '音视频',      exts: ['mp4','avi','mkv','mov','wmv','flv','webm','mp3','wav','flac','aac','ogg','m4a','opus'] },
  { label: '可执行 / 安装包', exts: ['exe','msi','deb','rpm','dmg','appimage','pkg','apk'] },
]

const fileTypeFilter = ref(0)

// ─── 显示模式（全部 / 仅文件 / 仅目录）────────────────────────────────────────

type ShowMode = 'all' | 'files' | 'dirs'
const showMode = ref<ShowMode>('all')

const currentPath  = ref('')
const pathInput    = ref('')
const files        = ref<LocalFile[]>([])
const loading      = ref(false)
const listError    = ref('')
const homePath     = ref('')
const roots        = ref<string[]>([])
const quickDirs    = ref<QuickDir[]>([])
const listRef      = ref<HTMLDivElement>()

// 历史（后退/前进）
const history    = ref<string[]>([])
const historyIdx = ref(-1)
const canBack    = computed(() => historyIdx.value > 0)
const canFwd     = computed(() => historyIdx.value < history.value.length - 1)

// ─── 选择状态 ─────────────────────────────────────────────────────────────────

const selectedPaths = ref<Set<string>>(new Set())
const anchorPath    = ref<string | null>(null)
const focusedIdx    = ref(-1)
const selInput      = ref('')

const selLabelText = computed(() => {
  if (selectedPaths.value.size === 0) return '文件/目录'
  const paths = [...selectedPaths.value]
  const allDirs = paths.every(p => sortedFiles.value.find(f => f.path === p)?.is_dir)
  return allDirs ? '目录' : '文件'
})

const canConfirm = computed(() => selectedPaths.value.size > 0 || selInput.value.trim() !== '')

watch(selectedPaths, (set) => {
  const names = [...set].map(p => p.replace(/\\/g, '/').split('/').pop() ?? p)
  selInput.value = names.join('  ')
}, { deep: true })

// ─── 排序 + 过滤后的文件列表 ──────────────────────────────────────────────────

const sortedFiles = computed<LocalFile[]>(() => {
  // 1. 显示模式过滤
  let list: LocalFile[] = files.value
  if (showMode.value === 'files') {
    list = list.filter(f => !f.is_dir)
  } else if (showMode.value === 'dirs') {
    list = list.filter(f => f.is_dir)
  }

  // 2. 文件类型过滤（仅目录模式时跳过，目录本身不受扩展名约束）
  const filter = FILE_TYPE_FILTERS[fileTypeFilter.value]
  if (filter.exts !== null && showMode.value !== 'dirs') {
    const extsSet = new Set(filter.exts)
    list = list.filter(f => {
      if (f.is_dir) return true
      const ext = f.name.split('.').pop()?.toLowerCase() ?? ''
      return extsSet.has(ext)
    })
  }

  // 3. 动态排序（混合模式下目录置顶）
  const mul = sortDir.value === 'asc' ? 1 : -1
  return [...list].sort((a, b) => {
    if (showMode.value === 'all' && a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1
    switch (sortBy.value) {
      case 'name':
        return mul * a.name.localeCompare(b.name, 'zh-CN')
      case 'size':
        return mul * (a.size - b.size)
      case 'type': {
        const ea = a.name.split('.').pop()?.toLowerCase() ?? ''
        const eb = b.name.split('.').pop()?.toLowerCase() ?? ''
        const res = ea.localeCompare(eb, 'zh-CN')
        return res !== 0 ? mul * res : a.name.localeCompare(b.name, 'zh-CN')
      }
      case 'mod_time':
        return mul * a.mod_time.localeCompare(b.mod_time)
      default:
        return 0
    }
  })
})

// ─── 导航 ─────────────────────────────────────────────────────────────────────

const LAST_PATH_KEY = 'gssh-lfp-last-path'

async function navigateTo(path: string, pushHistory = true) {
  if (!path) return
  loading.value = true
  listError.value = ''
  try {
    const result = await LocalList(path)
    files.value     = result as LocalFile[]
    currentPath.value = path
    pathInput.value   = path
    try { localStorage.setItem(LAST_PATH_KEY, path) } catch {}
    clearSelection()
    if (pushHistory) {
      history.value.splice(historyIdx.value + 1)
      history.value.push(path)
      historyIdx.value = history.value.length - 1
    }
  } catch (e: any) {
    listError.value = String(e)
  } finally {
    loading.value = false
  }
}

function commitPath() {
  const p = pathInput.value.trim()
  if (p && p !== currentPath.value) navigateTo(p)
}

function goBack() {
  if (!canBack.value) return
  historyIdx.value--
  navigateTo(history.value[historyIdx.value], false)
}

function goFwd() {
  if (!canFwd.value) return
  historyIdx.value++
  navigateTo(history.value[historyIdx.value], false)
}

function goUp() {
  const p = currentPath.value.replace(/[/\\]+$/, '')
  const lastSep = Math.max(p.lastIndexOf('/'), p.lastIndexOf('\\'))
  if (lastSep <= 0) {
    // 到达根（Linux /） 或无法再上
    if (p !== currentPath.value) navigateTo(p)
    return
  }
  let parent = p.slice(0, lastSep) || '/'
  // Windows: "C:" → "C:\"
  if (/^[A-Za-z]:$/.test(parent)) parent += '\\'
  navigateTo(parent)
}

// ─── 选择逻辑 ─────────────────────────────────────────────────────────────────

function clearSelection() {
  selectedPaths.value = new Set()
  anchorPath.value    = null
  focusedIdx.value    = -1
  selInput.value      = ''
}

function onRowClick(f: LocalFile, idx: number, e: MouseEvent) {
  focusedIdx.value = idx
  if (e.ctrlKey || e.metaKey) {
    const next = new Set(selectedPaths.value)
    if (next.has(f.path)) next.delete(f.path)
    else { next.add(f.path); anchorPath.value = f.path }
    selectedPaths.value = next
  } else if (e.shiftKey && anchorPath.value) {
    const anchorIdx = sortedFiles.value.findIndex(x => x.path === anchorPath.value)
    if (anchorIdx >= 0) {
      const lo = Math.min(anchorIdx, idx), hi = Math.max(anchorIdx, idx)
      selectedPaths.value = new Set(sortedFiles.value.slice(lo, hi + 1).map(x => x.path))
    }
  } else {
    selectedPaths.value = new Set([f.path])
    anchorPath.value    = f.path
  }
  listRef.value?.focus({ preventScroll: true })
}

function onDblClick(f: LocalFile) {
  if (f.is_dir) {
    navigateTo(f.path)
  } else {
    // 双击文件：确认
    selectedPaths.value = new Set([f.path])
    nextTick(confirm)
  }
}

// ─── 键盘导航 ─────────────────────────────────────────────────────────────────

function onKeydown(e: KeyboardEvent) {
  const files = sortedFiles.value
  if (e.altKey && e.key === 'ArrowLeft')  { e.preventDefault(); goBack(); return }
  if (e.altKey && e.key === 'ArrowRight') { e.preventDefault(); goFwd();  return }
  if (!files.length) return
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    e.preventDefault()
    const next = e.key === 'ArrowDown'
      ? Math.min(files.length - 1, focusedIdx.value < 0 ? 0 : focusedIdx.value + 1)
      : Math.max(0, focusedIdx.value < 0 ? 0 : focusedIdx.value - 1)
    focusedIdx.value = next
    const f = files[next]
    if (e.shiftKey) selectedPaths.value = new Set([...selectedPaths.value, f.path])
    else { selectedPaths.value = new Set([f.path]); anchorPath.value = f.path }
    scrollFocused()
  } else if (e.key === 'Enter' && focusedIdx.value >= 0) {
    const f = files[focusedIdx.value]
    if (f.is_dir) navigateTo(f.path)
    else confirm()
  } else if (e.key === 'Backspace') {
    goUp()
  } else if (e.key === 'Escape') {
    clearSelection()
  } else if ((e.ctrlKey || e.metaKey) && (e.key === 'a' || e.key === 'A')) {
    e.preventDefault()
    selectedPaths.value = new Set(files.map(f => f.path))
  }
}

function scrollFocused() {
  nextTick(() => {
    const el = listRef.value?.querySelector(`tr[data-idx="${focusedIdx.value}"]`) as HTMLElement | null
    el?.scrollIntoView({ block: 'nearest' })
  })
}

// ─── selInput 回车：直接确认输入的路径 ───────────────────────────────────────

function onSelEnter() {
  const raw = selInput.value.trim()
  if (!raw) return
  // 尝试当作当前目录下的相对名称
  const joined = currentPath.value.replace(/[/\\]+$/, '') +
    (currentPath.value.includes('\\') ? '\\' : '/') + raw
  const match = sortedFiles.value.find(
    f => f.name === raw || f.path === raw || f.path === joined
  )
  if (match) {
    selectedPaths.value = new Set([match.path])
    confirm()
  } else {
    // 假设是绝对路径或合法路径，直接确认
    resolveRef?.([raw])
    resolveRef = null
    visible.value = false
  }
}

// ─── 格式化工具 ───────────────────────────────────────────────────────────────

function fmtSize(bytes: number): string {
  if (bytes < 1024)        return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 ** 3)   return (bytes / 1024 / 1024).toFixed(1) + ' MB'
  return (bytes / 1024 ** 3).toFixed(2) + ' GB'
}

function fmtTime(t: string): string {
  if (!t) return ''
  try {
    const d = new Date(t)
    const y = d.getFullYear()
    const m = String(d.getMonth() + 1).padStart(2, '0')
    const day = String(d.getDate()).padStart(2, '0')
    const h = String(d.getHours()).padStart(2, '0')
    const min = String(d.getMinutes()).padStart(2, '0')
    return `${y}/${m}/${day} ${h}:${min}`
  } catch { return '' }
}

const EXT_EMOJI: Record<string, string> = {
  txt: '📝', md: '📝', log: '📝', csv: '📊', xlsx: '📊', xls: '📊',
  zip: '📦', tar: '📦', gz: '📦', rar: '📦', '7z': '📦',
  jpg: '🖼', jpeg: '🖼', png: '🖼', gif: '🖼', webp: '🖼', svg: '🖼',
  mp4: '🎬', avi: '🎬', mov: '🎬', mkv: '🎬',
  mp3: '🎵', wav: '🎵', flac: '🎵',
  pdf: '📄', doc: '📄', docx: '📄',
  exe: '⚙', msi: '⚙', sh: '⚙', bat: '⚙', cmd: '⚙',
  iso: '💿', img: '💿',
  js: '📜', ts: '📜', py: '📜', go: '📜', rs: '📜', vue: '📜',
}

function fileEmoji(name: string): string {
  const ext = name.split('.').pop()?.toLowerCase() ?? ''
  return EXT_EMOJI[ext] ?? '📄'
}

function extLabel(name: string): string {
  const ext = name.split('.').pop()?.toLowerCase() ?? ''
  return ext ? ext.toUpperCase() + ' 文件' : '文件'
}

// ─── 初始化 ───────────────────────────────────────────────────────────────────

async function resetState() {
  clearSelection()
  history.value      = []
  historyIdx.value   = -1
  listError.value    = ''
  files.value        = []
  sortBy.value         = 'mod_time'
  sortDir.value        = 'desc'
  fileTypeFilter.value = 0
  showMode.value       = 'all'
  if (!homePath.value) {
    homePath.value = await LocalHomePath().catch(() => '')
  }
  if (!roots.value.length) {
    roots.value = await LocalListRoots().catch(() => [])
  }
  if (!quickDirs.value.length) {
    quickDirs.value = await LocalQuickDirs().catch(() => [])
  }
  // 恢复上次路径，失败则回退到主目录
  const savedPath = localStorage.getItem(LAST_PATH_KEY)
  if (savedPath) {
    try { await navigateTo(savedPath); return } catch {}
  }
  await navigateTo(homePath.value)
}
</script>

<style>
/* 遮罩层 */
.lfp-mask {
  position: fixed;
  inset: 0;
  z-index: 800;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0,0,0,0.55);
}

/* 对话框主体 */
.lfp-dialog {
  width: 860px;
  height: 520px;
  max-width: 96vw;
  max-height: 90vh;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: 0 16px 48px rgba(0,0,0,0.6);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  user-select: none;
}

/* 标题栏 */
.lfp-titlebar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 12px;
  height: 36px;
  background: var(--bg-panel, var(--bg-secondary));
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
  font-size: 13px;
  font-weight: 600;
  cursor: grab;
}

.lfp-close {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 12px;
  cursor: pointer;
  padding: 3px 6px;
  border-radius: 4px;
  line-height: 1;
}
.lfp-close:hover { background: rgba(248,81,73,0.15); color: var(--danger); }

/* Look in 行 */
.lfp-lookin {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 10px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.lfp-lookin-label {
  font-size: 12px;
  color: var(--text-muted);
  white-space: nowrap;
  flex-shrink: 0;
}

.lfp-path-input {
  flex: 1;
  min-width: 0;
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-primary);
  font-size: 12px;
  padding: 4px 8px;
  outline: none;
  font-family: 'Cascadia Code', Consolas, monospace;
}
.lfp-path-input:focus { border-color: var(--accent); }

.lfp-nav-btn {
  background: var(--bg-primary);
  border: 1px solid var(--border);
  color: var(--text-muted);
  border-radius: 4px;
  padding: 3px 8px;
  cursor: pointer;
  font-size: 11px;
  flex-shrink: 0;
  transition: color 0.12s, background 0.12s;
}
.lfp-nav-btn:hover:not(:disabled) { color: var(--text-primary); background: rgba(255,255,255,0.08); }
.lfp-nav-btn:disabled { opacity: 0.3; cursor: not-allowed; }

/* 主体 */
.lfp-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

/* 左侧栏 */
.lfp-sidebar {
  width: 130px;
  flex-shrink: 0;
  border-right: 1px solid var(--border);
  overflow-y: auto;
  padding: 6px 0;
  background: var(--bg-secondary);
}

.lfp-sidebar-item {
  padding: 7px 12px;
  font-size: 12px;
  color: var(--text-muted);
  cursor: pointer;
  border-radius: 4px;
  margin: 1px 4px;
  transition: background 0.1s, color 0.1s;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.lfp-sidebar-item:hover { background: rgba(255,255,255,0.06); color: var(--text-primary); }
.lfp-sidebar-item.active { background: rgba(88,166,255,0.12); color: var(--accent); }

.lfp-sidebar-sep { height: 1px; background: var(--border); margin: 5px 8px; }

/* 文件列表 */
.lfp-list {
  flex: 1;
  overflow: auto;
  outline: none;
}

.lfp-status {
  padding: 20px;
  text-align: center;
  color: var(--text-muted);
  font-size: 12px;
}
.lfp-status.lfp-error { color: var(--danger); }

.lfp-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.lfp-table thead tr {
  position: sticky;
  top: 0;
  background: var(--bg-secondary);
  z-index: 1;
}

.lfp-table th {
  padding: 5px 8px;
  text-align: left;
  font-weight: 600;
  color: var(--text-muted);
  border-bottom: 1px solid var(--border);
  font-size: 11px;
}

.lfp-th-sort {
  cursor: pointer;
  user-select: none;
  white-space: nowrap;
}
.lfp-th-sort:hover { color: var(--text-primary); }

.lfp-sort-arrow {
  color: var(--accent);
  font-size: 11px;
}

.lfp-table tbody tr {
  cursor: pointer;
  transition: background 0.08s;
}
.lfp-table tbody tr:hover          { background: rgba(255,255,255,0.04); }
.lfp-table tbody tr.selected       { background: rgba(88,166,255,0.18); }
.lfp-table tbody tr.focused:not(.selected)  { background: rgba(255,255,255,0.04); outline: 1px solid rgba(88,166,255,0.25); outline-offset:-1px; }
.lfp-table tbody tr.selected.focused       { outline: 1px solid rgba(88,166,255,0.5); outline-offset:-1px; }

.lfp-table td {
  padding: 4px 8px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  border-bottom: 1px solid rgba(255,255,255,0.03);
}

.lfp-col-name { display: flex; align-items: center; gap: 5px; }
.lfp-file-icon { font-size: 14px; flex-shrink: 0; line-height: 1; }
.lfp-col-num { color: var(--text-muted); }

.lfp-empty { text-align: center; color: var(--text-muted); font-style: italic; padding: 20px; }

/* 底部 */
.lfp-footer {
  border-top: 1px solid var(--border);
  padding: 8px 12px;
  background: var(--bg-secondary);
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex-shrink: 0;
}

.lfp-sel-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.lfp-sel-label {
  font-size: 12px;
  color: var(--text-muted);
  white-space: nowrap;
  flex-shrink: 0;
  min-width: 56px;
  text-align: right;
}

.lfp-sel-input {
  flex: 1;
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-primary);
  font-size: 12px;
  padding: 4px 8px;
  outline: none;
}
.lfp-sel-input:focus { border-color: var(--accent); }

.lfp-type-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.lfp-type-select {
  flex: 1;
  min-width: 0;
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-primary);
  font-size: 12px;
  padding: 4px 8px;
  outline: none;
  cursor: pointer;
  transition: border-color 0.12s;
}
.lfp-type-select:focus { border-color: var(--accent); }
.lfp-type-select:disabled { opacity: 0.4; cursor: not-allowed; }

/* 显示模式切换按钮组 */
.lfp-show-toggle {
  display: flex;
  flex-shrink: 0;
}
.lfp-tog {
  padding: 3px 10px;
  font-size: 11px;
  background: var(--bg-primary);
  border: 1px solid var(--border);
  color: var(--text-muted);
  cursor: pointer;
  transition: background 0.1s, color 0.1s;
  line-height: 1.6;
}
.lfp-tog:first-child { border-radius: 4px 0 0 4px; }
.lfp-tog:last-child  { border-radius: 0 4px 4px 0; }
.lfp-tog:not(:first-child) { border-left: none; }
.lfp-tog:hover { color: var(--text-primary); background: rgba(255,255,255,0.07); }
.lfp-tog.active { background: var(--accent); border-color: var(--accent); color: #fff; }

.lfp-btn-row {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
}

.lfp-btn {
  padding: 5px 18px;
  border-radius: 5px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-primary);
  font-size: 12px;
  cursor: pointer;
  transition: background 0.12s;
}
.lfp-btn:hover:not(:disabled) { background: rgba(255,255,255,0.07); }
.lfp-btn:disabled { opacity: 0.35; cursor: not-allowed; }

.lfp-btn-primary {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
  font-weight: 600;
}
.lfp-btn-primary:hover:not(:disabled) { filter: brightness(1.1); }
</style>
