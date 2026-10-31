<template>
  <div
    class="fm-root"
    :class="{ 'drop-active': remoteDropActive }"
    style="--wails-drop-target: drop"
    @dragover.prevent="remoteDropActive = true"
    @dragleave="remoteDropActive = false"
    @drop.prevent
  >
<!-- ── 面板头 ── -->
    <div class="fm-head">
      <i class="ph ph-folders fm-head-icon"></i>
      <span class="fm-head-title">文件浏览</span>
      <div class="fm-spring"></div>
      <span class="fm-sftp-tag">SFTP</span>
    </div>

    <!-- ── 工具栏（两行） ── -->
    <div class="fm-toolbar">

      <!-- 第一行：路径框 + 导航按钮 -->
      <div class="fm-nav-row">
        <div class="fm-path-box">
          <i class="ph ph-house fm-path-icon"></i>
          <input
            ref="pathInputRef"
            v-model="pathInput"
            class="fm-path-input mono"
            spellcheck="false"
            title="单击编辑路径，Enter 确认，Esc 取消"
            @focus="onPathFocus"
            @keydown.enter.prevent="commitPath"
            @keydown.esc.prevent="cancelEditPath"
            @blur="commitPath"
          />
        </div>
        <button class="fm-nav-btn" title="上级目录 (Backspace)" @click="goUp"><i class="ph ph-arrow-up"></i></button>
        <button class="fm-nav-btn" title="刷新" @click="loadRemote()"><i class="ph ph-arrows-clockwise"></i></button>
      </div>

      <!-- 第二行：过滤 + 隐藏文件开关 -->
      <div class="fm-filter-row">
        <div class="fm-filter-box">
          <i class="ph ph-magnifying-glass fm-filter-icon"></i>
          <input
            v-model="filterQuery"
            class="fm-filter-input"
            placeholder="按名称过滤…"
            title="按名称过滤 (Esc 清除)"
            @keydown.esc.stop="filterQuery = ''"
          />
        </div>
        <div
          class="fm-hidden-toggle"
          :class="{ active: showHidden }"
          title="显示 / 隐藏 以点开头的文件"
          @click="showHidden = !showHidden"
        >
          <span class="fm-switch"><span class="fm-switch-knob"></span></span>
          <span class="fm-switch-label">隐藏文件</span>
        </div>
      </div>

    </div>

    <!-- ── 文件列表 ── -->
    <div class="fm-body">
      <div
        ref="listBodyRef"
        class="fm-list"
        tabindex="0"
        title="↑↓ 导航 · Enter 打开 · Del 删除 · Ctrl+A 全选 · Esc 取消"
        @scroll.passive="onListScroll"
        @keydown="onListKeydown"
        @mousedown="onListMousedown"
        @contextmenu.prevent="openCtxMenu($event, null)"
        @click.self="onListClickSelf"
      >
        <!-- 拖拽调整列宽时的全屏覆盖层，固定 col-resize 光标 -->
        <div v-if="isResizing" class="col-resize-overlay" />

        <!-- 框选矩形（fixed 坐标，pointer-events:none） -->
        <div v-if="dragSel.active" class="drag-sel-box" :style="dragSelStyle" />

        <!-- 列头拖拽时跟随光标的浮动标签（fixed，pointer-events:none 不挡命中测试） -->
        <div
          v-if="colDrag.dragging && colDrag.col"
          class="col-drag-ghost"
          :style="{ left: colDrag.x + 12 + 'px', top: colDrag.y + 12 + 'px' }"
        >
          <span class="col-drag-ghost-grip">⣿</span>
          {{ FM_COLUMN_LABELS[colDrag.col as FMColumn] }}
        </div>

        <div v-if="remoteLoading" class="fm-loading">加载中…</div>
        <div v-else-if="remoteError" class="fm-error">{{ remoteError }}</div>
        <table v-else class="fm-table">
          <colgroup>
            <col style="width: 28px" />
            <col
              v-for="col in visibleColumns"
              :key="col"
              :style="{ width: colWidths[col] + 'px' }"
            />
            <!-- spacer: absorbs remaining width so named cols stay at exact px -->
            <col />
          </colgroup>
          <thead>
            <tr>
              <th class="col-icon"></th>
              <th
                v-for="(col, idx) in visibleColumns"
                :key="col"
                class="col-sortable"
                :class="{
                  'sort-active': sortBy === col,
                  'col-dragging': colDrag.dragging && colDrag.fromIdx === idx,
                  'col-drag-over': colDrag.dragging && colDrag.overIdx === idx && colDrag.fromIdx !== idx,
                }"
                :data-col-idx="idx"
                @mousedown="onColHeaderDown($event, idx, col)"
              >
                <span class="col-label">{{ FM_COLUMN_LABELS[col] }}</span>
                <span class="sort-arrow">{{ sortBy === col ? (sortDir === 'asc' ? '↑' : '↓') : '' }}</span>
                <!-- 拖拽 handle：@mousedown.stop.prevent 既阻止冒泡触发排序，也阻止列头拖拽启动 -->
                <span
                  class="col-resize-handle"
                  @mousedown.stop.prevent="onColResizeStart($event, col)"
                  @dblclick.stop.prevent="resetColWidth(col)"
                />
              </th>
              <th class="col-spacer"></th>
            </tr>
          </thead>
          <tbody>
            <!-- 虚拟滚动：顶部占位 -->
            <tr v-if="virtTopSpacer > 0" class="fm-vspacer" :style="{ height: virtTopSpacer + 'px' }">
              <td colspan="99"></td>
            </tr>

            <tr
              v-for="(f, vi) in virtFiles"
              :key="f.name"
              :data-idx="virtRange.start + vi"
              :class="{
                selected: selectedNames.has(f.name),
                focused:  focusedIdx === virtRange.start + vi,
              }"
              @click="onRowClick($event, f, virtRange.start + vi)"
              @dblclick="onDblClick(f)"
              @mouseenter="focusedIdx = virtRange.start + vi"
              @contextmenu.prevent.stop="openCtxMenu($event, f)"
            >
              <td class="col-icon">
                <i v-if="f.is_dir" class="ph-fill ph-folder fm-icon-folder"></i>
                <i v-else class="ph fm-icon-file" :class="fileIconClass(f)"></i>
              </td>
              <td v-for="col in visibleColumns" :key="col" :class="'fm-col-' + col">
                <template v-if="col === 'name'">
                  <span :title="f.name">{{ f.name }}</span>
                </template>
                <template v-else-if="col === 'size'">{{ f.is_dir ? '' : fmtSize(f.size) }}</template>
                <template v-else-if="col === 'mod_time'">{{ fmtTime(f.mod_time) }}</template>
                <template v-else>{{ (f as any)[col] }}</template>
              </td>
              <td class="col-spacer"></td>
            </tr>
            <!-- 虚拟滚动：底部占位 -->
            <tr v-if="virtBottomSpacer > 0" class="fm-vspacer" :style="{ height: virtBottomSpacer + 'px' }">
              <td colspan="99"></td>
            </tr>

            <tr v-if="!remoteLoading && filteredFiles.length === 0">
              <td class="fm-empty-cell" colspan="99">
                {{ filterQuery ? '没有匹配的文件' : '（空目录）' }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- ── 独立预览弹窗 ── -->
    <FilePreviewModal
      :state="preview"
      :file-icon="fileIcon"
      :fmt-size="fmtSize"
      @close="preview.visible = false"
    />

    <!-- ── 底栏：状态 + 拖拽提示 ── -->
    <div class="fm-footer">
      <span class="fm-status-text">{{ statusText }}</span>
      <div class="fm-spring"></div>
      <span class="fm-drop-hint"><i class="ph ph-upload-simple"></i>拖拽上传</span>
    </div>

    <!-- ── 右键菜单 ── -->
    <div
      v-if="ctxMenu.visible"
      class="ctx-menu"
      :style="{ top: ctxMenu.y + 'px', left: ctxMenu.x + 'px' }"
      @click.stop
    >
      <!-- 多选模式 -->
      <template v-if="selectedNames.size > 1">
        <div v-if="downloadableCount > 0" class="ctx-item" @click="onCtxBatchDownload">
          下载 {{ downloadableCount }} 个文件
        </div>
        <div class="ctx-item" @click="onCtxBatchMove">移动 {{ selectedNames.size }} 项到…</div>
        <div class="ctx-item danger" @click="onCtxBatchDelete">
          删除 {{ selectedNames.size }} 项
        </div>
        <div class="ctx-divider" />
        <div class="ctx-item" @click="onCtxNewFile">📄 新建文件</div>
        <div class="ctx-item" @click="onCtxMkdir">📁 新建目录</div>
        <div class="ctx-item" @click="onCtxUpload">⬆ 上传</div>
        <div class="ctx-item" @click="onCtxRefresh">刷新</div>
      </template>
      <!-- 单选模式 -->
      <template v-else-if="ctxSingleFile">
        <div v-if="!ctxSingleFile.is_dir" class="ctx-item" @click="onCtxPreview">预览</div>
        <div class="ctx-item" @click="onCtxDownload">下载{{ ctxSingleFile.is_dir ? '目录' : '' }}</div>
        <div v-if="!ctxSingleFile.is_dir" class="ctx-item" @click="onCtxEdit">用本地编辑器打开</div>
        <div class="ctx-item" @click="onCtxRename">重命名</div>
        <div class="ctx-item" @click="onCtxMove">移动到…</div>
        <div class="ctx-item" @click="onCtxChmod">修改权限</div>
        <div class="ctx-item" @click="onCtxCopyPath">复制路径</div>
        <div class="ctx-divider" />
        <div class="ctx-item danger" @click="onCtxDelete">删除</div>
        <div class="ctx-divider" />
        <div class="ctx-item" @click="onCtxNewFile">📄 新建文件</div>
        <div class="ctx-item" @click="onCtxMkdir">📁 新建目录</div>
        <div class="ctx-item" @click="onCtxUpload">⬆ 上传</div>
        <div class="ctx-item" @click="onCtxRefresh">刷新</div>
      </template>
      <!-- 空白区域 -->
      <template v-else>
        <div class="ctx-item" @click="onCtxNewFile">📄 新建文件</div>
        <div class="ctx-item" @click="onCtxMkdir">📁 新建目录</div>
        <div class="ctx-item" @click="onCtxUpload">⬆ 上传</div>
        <div class="ctx-item" @click="onCtxRefresh">刷新</div>
      </template>
    </div>
    <div v-if="ctxMenu.visible" class="ctx-overlay" @click="ctxMenu.visible = false" />

    <!-- ── 本地文件选择器 ── -->
    <LocalFilePicker ref="localFilePickerRef" />

    <!-- ── chmod 弹窗 ── -->
    <div v-if="chmodDialog.visible" class="dl-overlay" @click.self="chmodDialog.visible = false">
      <div class="dl-dialog chmod-dialog">
        <div class="dl-dialog-header">
          <span class="dl-dialog-title">🔒 修改权限</span>
          <button class="dl-close-btn" @click="chmodDialog.visible = false">✕</button>
        </div>
        <div class="dl-dialog-body">
          <div class="dl-file-name">{{ chmodDialog.name }}</div>

          <!-- rwx 矩阵 -->
          <div class="chmod-matrix">
            <div class="chmod-row-header"><span></span><span>读(r)</span><span>写(w)</span><span>执行(x)</span></div>
            <div v-for="(row, ri) in chmodRows" :key="ri" class="chmod-row">
              <span class="chmod-label">{{ row.label }}</span>
              <label v-for="(bit, bi) in row.bits" :key="bi" class="chmod-cb">
                <input type="checkbox" :checked="bit as boolean" @change="onChmodBit(ri, bi, ($event.target as HTMLInputElement).checked)" />
              </label>
            </div>
          </div>

          <!-- 八进制显示 -->
          <div class="chmod-octal-row">
            <span class="chmod-octal-label">八进制：</span>
            <input v-model="chmodOctal" class="chmod-octal-input" maxlength="4"
              @input="onChmodOctalInput" placeholder="755" />
          </div>
        </div>
        <div class="dl-dialog-footer">
          <button class="dl-btn" @click="chmodDialog.visible = false">取消</button>
          <button class="dl-btn dl-btn-primary" :disabled="chmodSaving" @click="commitChmod">
            {{ chmodSaving ? '保存中…' : '确定' }}
          </button>
        </div>
      </div>
    </div>

    <!-- ── 下载路径对话框 ── -->
    <div v-if="dlDialog.visible" class="dl-overlay">
      <div class="dl-dialog">
        <div class="dl-dialog-header">
          <span class="dl-dialog-title">⬇ 下载</span>
          <button class="dl-close-btn" @click="dlDialogCancel">✕</button>
        </div>
        <div class="dl-dialog-body">
          <div class="dl-file-name" :title="dlDialog.fileName">{{ dlDialog.fileName }}</div>
          <div class="dl-path-row">
            <input
              v-model="dlDialog.dirPath"
              class="dl-path-input"
              placeholder="选择下载目录…"
              spellcheck="false"
            />
            <button class="fm-btn" @click="browseDlDir">…</button>
          </div>
        </div>
        <div class="dl-dialog-footer">
          <button class="fm-btn" @click="dlDialogCancel">取消</button>
          <button
            class="fm-btn fm-btn-primary"
            :disabled="!dlDialog.dirPath.trim()"
            @click="dlDialogConfirm"
          >开始下载</button>
        </div>
      </div>
    </div>

    <!-- ── 编辑器变动提示 ── -->
    <div v-if="editorPrompt.visible" class="editor-prompt-overlay">
      <div class="editor-prompt">
        <div class="ep-title">文件已修改</div>
        <div class="ep-body">
          本地编辑的 <strong>{{ editorPrompt.name }}</strong> 已保存，是否上传到服务器？
        </div>
        <div class="ep-actions">
          <button class="fm-btn fm-btn-primary" @click="confirmEditorUpload">上传</button>
          <button class="fm-btn" @click="dismissEditorPrompt">跳过</button>
          <button class="fm-btn" style="margin-left:auto" @click="stopEditorWatch">停止监视</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { EventsOn, EventsOff, OnFileDrop, OnFileDropOff } from '../../wailsjs/runtime/runtime'
import { useSettingsStore, FM_ALL_COLUMNS, FM_COLUMN_LABELS, type FMColumn } from '../stores/settings'
import { useTabsStore } from '../stores/tabs'
import { useDialogStore } from '../stores/dialogs'
import { useTransfersStore } from '../stores/transfers'
import FilePreviewModal, { type PreviewState } from './FilePreviewModal.vue'
import LocalFilePicker from './LocalFilePicker.vue'
import {
  SFTPList, SFTPHomePath,
  SFTPUpload, SFTPDownload, SFTPUploadDir, SFTPDownloadDir,
  SFTPDeleteWithProgress, SFTPMkdir, SFTPCreateFile, SFTPRename,
  SFTPReadFile, SFTPChmod, SFTPStat,
  OpenWithEditor, UploadEditedFile, UploadEditedFileSudo, StopEditorWatch,
  OpenDirectoryDialog, GetDownloadsDir, LocalIsDir,
} from '../../wailsjs/go/main/App'

const props = defineProps<{ connId: string }>()

// ─── Types ────────────────────────────────────────────────────────────────────

interface RemoteFile {
  name: string
  size: number
  mode: string
  mod_time: string
  is_dir: boolean
  owner: string
  group: string
}

// ─── Settings & sorting ───────────────────────────────────────────────────────

const settingsStore   = useSettingsStore()
const tabsStore       = useTabsStore()
const dlg             = useDialogStore()
const transfersStore  = useTransfersStore()

const sortBy  = ref<FMColumn>('name')
const sortDir = ref<'asc' | 'desc'>('asc')

function toggleSort(col: FMColumn) {
  if (sortBy.value === col) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortBy.value = col
    sortDir.value = 'asc'
  }
}

// ─── 列头拖拽重排（自定义指针拖拽，不用原生 HTML5 drag）──────────
// mousedown 阻断冒泡（避免触发列表框选/文件拖动），越过 5px 阈值才算拖动，
// 未越过则视为单击 → 排序。拖动时用浮动标签 + 落点竖线做视觉反馈。
const colDrag = reactive({
  pending:  false,             // 已按下、尚未判定为拖动
  dragging: false,
  fromIdx:  -1,
  overIdx:  -1,
  col:      '' as FMColumn | '',
  startX:   0,
  startY:   0,
  x:        0,                 // 浮动标签跟随的光标坐标
  y:        0,
})
let _colDragCleanup: (() => void) | null = null

function onColHeaderDown(e: MouseEvent, idx: number, col: FMColumn) {
  if (e.button !== 0) return
  e.stopPropagation()          // 关键：不冒泡到 onListMousedown（框选）
  colDrag.pending  = true
  colDrag.dragging = false
  colDrag.fromIdx  = idx
  colDrag.overIdx  = idx
  colDrag.col      = col
  colDrag.startX   = e.clientX
  colDrag.startY   = e.clientY

  const move = (ev: MouseEvent) => onColHeaderMove(ev)
  const up   = (ev: MouseEvent) => onColHeaderUp(ev, col)
  _colDragCleanup = () => {
    document.removeEventListener('mousemove', move)
    document.removeEventListener('mouseup',   up)
    document.body.style.userSelect = ''
    document.body.style.cursor     = ''
  }
  document.addEventListener('mousemove', move)
  document.addEventListener('mouseup',   up)
}

function onColHeaderMove(e: MouseEvent) {
  if (!colDrag.pending) return
  if (!colDrag.dragging) {
    const dx = e.clientX - colDrag.startX
    const dy = e.clientY - colDrag.startY
    if (Math.sqrt(dx * dx + dy * dy) < 5) return   // 阈值内不算拖动
    colDrag.dragging = true
    document.body.style.userSelect = 'none'
    document.body.style.cursor     = 'grabbing'
  }
  colDrag.x = e.clientX
  colDrag.y = e.clientY
  // 命中测试：光标下的列头（ghost 设了 pointer-events:none，不会挡住）
  const el = document.elementFromPoint(e.clientX, e.clientY)
  const th = el?.closest('th[data-col-idx]') as HTMLElement | null
  if (th && th.dataset.colIdx != null) {
    const oi = Number(th.dataset.colIdx)
    if (!Number.isNaN(oi)) colDrag.overIdx = oi
  }
}

function onColHeaderUp(_e: MouseEvent, col: FMColumn) {
  _colDragCleanup?.()
  _colDragCleanup = null

  const wasDragging = colDrag.dragging
  const from = colDrag.fromIdx
  const to   = colDrag.overIdx
  colDrag.pending  = false
  colDrag.dragging = false
  colDrag.fromIdx  = -1
  colDrag.overIdx  = -1
  colDrag.col      = ''

  if (wasDragging) {
    if (from >= 0 && to >= 0 && from !== to) {
      const cols = [...visibleColumns.value]
      const [moved] = cols.splice(from, 1)
      cols.splice(to, 0, moved)
      // fm_columns 保存的就是可见列顺序 → 持久化
      settingsStore.save({ ...settingsStore.settings, fm_columns: cols })
    }
  } else {
    toggleSort(col)            // 没拖动 = 单击 → 排序
  }
}

const visibleColumns = computed<FMColumn[]>(() => {
  const cols = settingsStore.settings.fm_columns
  const valid = cols.filter(c => (FM_ALL_COLUMNS as readonly string[]).includes(c)) as FMColumn[]
  return valid.length ? valid : Array.from(FM_ALL_COLUMNS)
})

const sortedFiles = computed(() => {
  const col = sortBy.value
  return [...remoteFiles.value].sort((a, b) => {
    if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1
    let va: string | number = a[col as keyof RemoteFile] as string | number
    let vb: string | number = b[col as keyof RemoteFile] as string | number
    if (col === 'size') { va = Number(va); vb = Number(vb) }
    const cmp = va < vb ? -1 : va > vb ? 1 : 0
    return sortDir.value === 'asc' ? cmp : -cmp
  })
})

// ─── Filtering ────────────────────────────────────────────────────────────────

const showHidden  = ref(false)
const filterQuery = ref('')

const filteredFiles = computed(() => {
  let files = sortedFiles.value
  if (!showHidden.value) {
    files = files.filter(f => !f.name.startsWith('.'))
  }
  const q = filterQuery.value.trim().toLowerCase()
  if (q) {
    files = files.filter(f => f.name.toLowerCase().includes(q))
  }
  return files
})

// ─── Virtual scroll ───────────────────────────────────────────────────────────

const VIRT_ROW_HEIGHT = 26   // px per row (padding 3+3 + border 1 + 13px text ≈ 26)
const VIRT_OVERSCAN   = 12   // extra rows above/below viewport

const listBodyRef      = ref<HTMLDivElement>()
const listScrollTop    = ref(0)
const listClientHeight = ref(400)   // initial guess; updated on mount + resize

function onListScroll(e: Event) {
  const el = e.currentTarget as HTMLElement
  listScrollTop.value    = el.scrollTop
  listClientHeight.value = el.clientHeight
}

const remotePath       = ref('/')
const remoteFiles      = ref<RemoteFile[]>([])
const remoteLoading    = ref(false)
const remoteError      = ref('')
const remoteDropActive = ref(false)

// ─── Multi-select ─────────────────────────────────────────────────────────────

const selectedNames = ref<Set<string>>(new Set())
const anchorName    = ref<string | null>(null)
const focusedIdx    = ref(-1)

const selectedFiles = computed(() =>
  filteredFiles.value.filter(f => selectedNames.value.has(f.name))
)

/** For single-selection context menu ops */
const ctxSingleFile = computed<RemoteFile | null>(() => {
  if (selectedNames.value.size !== 1) return null
  const name = [...selectedNames.value][0]
  return remoteFiles.value.find(f => f.name === name) ?? null
})

const downloadableCount = computed(() => selectedFiles.value.length)

function clearSelection() {
  selectedNames.value = new Set()
  anchorName.value = null
}

// ─── Navigation history ───────────────────────────────────────────────────────

const pathHistory = ref<string[]>([])
const historyIdx  = ref(-1)
const canGoBack   = computed(() => historyIdx.value > 0)
const canGoFwd    = computed(() => historyIdx.value < pathHistory.value.length - 1)

// ─── Virtual scroll computed ───────────────────────────────────────────────────

const virtRange = computed(() => {
  const total = filteredFiles.value.length
  const start = Math.max(0, Math.floor(listScrollTop.value / VIRT_ROW_HEIGHT) - VIRT_OVERSCAN)
  const end   = Math.min(total, Math.ceil((listScrollTop.value + listClientHeight.value) / VIRT_ROW_HEIGHT) + VIRT_OVERSCAN)
  return { start, end }
})
const virtFiles         = computed(() => filteredFiles.value.slice(virtRange.value.start, virtRange.value.end))
const virtTopSpacer     = computed(() => virtRange.value.start * VIRT_ROW_HEIGHT)
const virtBottomSpacer  = computed(() => Math.max(0, (filteredFiles.value.length - virtRange.value.end) * VIRT_ROW_HEIGHT))

/** User-initiated navigation — updates history stack */
async function navigateTo(path: string, pushHistory = true) {
  remoteLoading.value = true
  remoteError.value   = ''
  filterQuery.value   = ''
  try {
    const files = await SFTPList(props.connId, path)
    remoteFiles.value   = files
    remotePath.value    = path
    selectedNames.value = new Set()
    anchorName.value    = null
    focusedIdx.value    = -1
    if (pushHistory) {
      pathHistory.value.splice(historyIdx.value + 1)
      pathHistory.value.push(path)
      historyIdx.value = pathHistory.value.length - 1
    }
  } catch (e: any) {
    remoteError.value = String(e)
  } finally {
    remoteLoading.value = false
  }
}

function goBack() {
  if (!canGoBack.value) return
  historyIdx.value--
  navigateTo(pathHistory.value[historyIdx.value], false)
}

function goFwd() {
  if (!canGoFwd.value) return
  historyIdx.value++
  navigateTo(pathHistory.value[historyIdx.value], false)
}

function goUp() {
  const parts = remotePath.value.replace(/\\/g, '/').split('/').filter(Boolean)
  if (parts.length === 0) return
  parts.pop()
  navigateTo(('/' + parts.join('/')) || '/')
}

// ─── Path input (always-visible editable) ─────────────────────────────────────

const pathInput    = ref('/')
const pathInputRef = ref<HTMLInputElement>()

// Keep pathInput synced with remotePath when not focused
watch(remotePath, (newPath) => {
  if (document.activeElement !== pathInputRef.value) {
    pathInput.value = newPath
  }
})

function onPathFocus() {
  pathInput.value = remotePath.value
  nextTick(() => pathInputRef.value?.select())
}

function commitPath() {
  const p = pathInput.value.trim()
  if (p && p !== remotePath.value) {
    navigateTo(p)
  } else {
    pathInput.value = remotePath.value
  }
}

function cancelEditPath() {
  pathInput.value = remotePath.value
  pathInputRef.value?.blur()
}

// ─── Remote filesystem (refresh without pushing history) ──────────────────────

async function loadRemote(path?: string) {
  const p = path ?? remotePath.value
  remoteLoading.value = true
  remoteError.value   = ''
  try {
    remoteFiles.value   = await SFTPList(props.connId, p)
    remotePath.value    = p
    selectedNames.value = new Set()
    anchorName.value    = null
    focusedIdx.value    = -1
  } catch (e: any) {
    remoteError.value = String(e)
  } finally {
    remoteLoading.value = false
  }
}

function onDblClick(f: RemoteFile) {
  if (f.is_dir) navigateTo(remotePath.value.replace(/\/$/, '') + '/' + f.name)
  else loadPreview(f)
}

function onRowClick(e: MouseEvent, f: RemoteFile, idx: number) {
  // If a rubber-band drag just ended, don't let the follow-up click override the selection
  if (_hadDrag) return
  if (e.ctrlKey || e.metaKey) {
    // Toggle individual item
    const next = new Set(selectedNames.value)
    if (next.has(f.name)) {
      next.delete(f.name)
    } else {
      next.add(f.name)
      anchorName.value = f.name
    }
    selectedNames.value = next
  } else if (e.shiftKey && anchorName.value) {
    // Range selection from anchor
    const anchorIdx = filteredFiles.value.findIndex(x => x.name === anchorName.value)
    if (anchorIdx >= 0) {
      const lo = Math.min(anchorIdx, idx)
      const hi = Math.max(anchorIdx, idx)
      selectedNames.value = new Set(filteredFiles.value.slice(lo, hi + 1).map(x => x.name))
    }
  } else {
    // Single select
    selectedNames.value = new Set([f.name])
    anchorName.value = f.name
  }
  focusedIdx.value = idx
  listBodyRef.value?.focus({ preventScroll: true })
}

// ─── Keyboard navigation ──────────────────────────────────────────────────────

function onListKeydown(e: KeyboardEvent) {
  const files = filteredFiles.value

  // Alt+← / Alt+→: history navigation
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
    if (e.shiftKey) {
      const extended = new Set(selectedNames.value)
      extended.add(f.name)
      selectedNames.value = extended
    } else {
      selectedNames.value = new Set([f.name])
      anchorName.value = f.name
    }
    scrollFocusedIntoView()
    return
  }

  switch (e.key) {
    case 'Enter':
      if (focusedIdx.value >= 0) {
        const f = files[focusedIdx.value]
        if (f.is_dir) navigateTo(remotePath.value.replace(/\/$/, '') + '/' + f.name)
        else loadPreview(f)
      }
      break
    case 'Backspace':
      goUp()
      break
    case 'F2':
      e.preventDefault()
      if (focusedIdx.value >= 0) doRename(files[focusedIdx.value])
      break
    case 'Delete':
      if (selectedNames.value.size > 0) batchDelete()
      break
    case 'Escape':
      clearSelection()
      filterQuery.value = ''
      break
    case 'a':
    case 'A':
      if (e.ctrlKey || e.metaKey) {
        e.preventDefault()
        selectedNames.value = new Set(files.map(f => f.name))
      }
      break
  }
}

function scrollFocusedIntoView() {
  const idx = focusedIdx.value
  if (idx < 0 || !listBodyRef.value) return
  // If the row falls outside the rendered virtual window, scroll the container
  // so it becomes visible, then let nextTick render it and call scrollIntoView.
  const { start, end } = virtRange.value
  if (idx < start || idx >= end) {
    listBodyRef.value.scrollTop = Math.max(0, idx * VIRT_ROW_HEIGHT - listClientHeight.value / 2)
    listScrollTop.value = listBodyRef.value.scrollTop
  }
  nextTick(() => {
    const el = listBodyRef.value?.querySelector(
      `tr[data-idx="${idx}"]`
    ) as HTMLElement | null
    el?.scrollIntoView({ block: 'nearest' })
  })
}

// ─── Status bar ───────────────────────────────────────────────────────────────

const statusText = computed(() => {
  const total = filteredFiles.value.length
  const dirs  = filteredFiles.value.filter(f => f.is_dir).length
  const files = total - dirs
  const sel   = selectedNames.value.size
  let s = `${dirs} 目录  ${files} 文件`
  if (sel > 0) {
    const selNonDir  = selectedFiles.value.filter(f => !f.is_dir)
    const totalSize  = selNonDir.reduce((acc, f) => acc + f.size, 0)
    s += `　已选 ${sel} 项`
    if (selNonDir.length > 0) s += `（${fmtSize(totalSize)}）`
  }
  return s
})

// ─── File preview ─────────────────────────────────────────────────────────────

const IMAGE_EXTS = new Set(['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp', 'ico'])
const TEXT_EXTS  = new Set(['txt', 'md', 'json', 'yaml', 'yml', 'toml', 'ini', 'conf',
  'sh', 'bash', 'zsh', 'py', 'rb', 'go', 'ts', 'js', 'tsx', 'jsx', 'vue', 'html',
  'css', 'scss', 'xml', 'csv', 'log', 'env', 'gitignore', 'dockerfile', 'sql'])

function fileExt(name: string) { return name.split('.').pop()?.toLowerCase() ?? '' }

const B = (n: string) => `/icons/brands/${n}.svg`
const S = (n: string) => `/icons/solid/${n}.svg`

const FILE_ICON_MAP: Record<string, string> = {
  // 图片
  png: S('file-image'), jpg: S('file-image'), jpeg: S('file-image'),
  gif: S('file-image'), webp: S('file-image'), svg: S('file-image'),
  bmp: S('file-image'), ico: S('file-image'), tiff: S('file-image'), tif: S('file-image'),
  // 视频
  mp4: S('file-video'), mkv: S('file-video'), avi: S('file-video'),
  mov: S('file-video'), wmv: S('file-video'), flv: S('file-video'), webm: S('file-video'),
  // 音频
  mp3: S('file-audio'), wav: S('file-audio'), flac: S('file-audio'),
  aac: S('file-audio'), ogg: S('file-audio'), m4a: S('file-audio'), opus: S('file-audio'),
  // 压缩包
  zip: S('file-zipper'), tar: S('file-zipper'), gz: S('file-zipper'),
  bz2: S('file-zipper'), xz: S('file-zipper'), rar: S('file-zipper'),
  '7z': S('file-zipper'), zst: S('file-zipper'),
  // 文档
  pdf: S('file-pdf'),
  doc: S('file-word'), docx: S('file-word'),
  xls: S('file-excel'), xlsx: S('file-excel'), csv: S('file-csv'),
  ppt: S('file-powerpoint'), pptx: S('file-powerpoint'),
  // 编程语言 — 有官方品牌图标
  go: B('golang'),
  py: B('python'), pyw: B('python'), ipynb: B('python'),
  js: B('js'), mjs: B('js'), cjs: B('js'),
  ts: B('typescript'), mts: B('typescript'), cts: B('typescript'),
  jsx: B('react'), tsx: B('react'),
  vue: B('vuejs'),
  rs: B('rust'),
  java: B('java'), kt: B('java'), kts: B('java'), groovy: B('java'),
  php: B('php'),
  swift: B('swift'),
  dart: B('dart-lang'),
  r: B('r-project'), rmd: B('r-project'),
  erl: B('erlang'), hrl: B('erlang'),
  // 编程语言 — 无品牌图标，统一用代码文件图标
  rb: S('file-code'), rake: S('file-code'),
  scala: S('file-code'),
  c: S('file-code'), h: S('file-code'),
  cpp: S('file-code'), cc: S('file-code'), cxx: S('file-code'), hpp: S('file-code'), hxx: S('file-code'),
  cs: S('file-code'),
  lua: S('file-code'),
  ex: S('file-code'), exs: S('file-code'),
  clj: S('file-code'), cljs: S('file-code'),
  hs: S('file-code'), lhs: S('file-code'),
  nim: S('file-code'), zig: S('file-code'),
  // Shell / 脚本
  sh: S('terminal'), bash: S('terminal'), zsh: S('terminal'),
  fish: S('terminal'), ps1: S('terminal'), bat: S('terminal'), cmd: S('terminal'),
  // Web
  html: B('html5'), htm: B('html5'),
  css: B('css'), scss: B('sass'), sass: B('sass'), less: B('less'), styl: S('file-code'),
  // 配置 / 数据
  json: S('gear'), jsonc: S('gear'),
  yaml: S('gear'), yml: S('gear'),
  toml: S('gear'), xml: S('gear'), plist: S('gear'),
  ini: S('gear'), conf: S('gear'), cfg: S('gear'), config: S('gear'),
  env: S('key'),
  // 文本 / 文档
  txt: S('file-lines'),
  md: B('markdown'), mdx: B('markdown'), markdown: B('markdown'),
  rst: S('file-lines'), adoc: S('file-lines'), log: S('file-lines'),
  // 数据库
  sql: S('database'), db: S('database'), sqlite: S('database'), sqlite3: S('database'),
  // 安全 / 证书
  pem: S('lock'), cert: S('lock'), crt: S('lock'), p12: S('lock'), pfx: S('lock'),
  key: S('key'), pub: S('key'),
  // 可执行 / 库
  exe: S('gear'), msi: S('gear'), bin: S('file'), out: S('file'), run: S('file'),
  so: S('gear'), dll: S('gear'), dylib: S('gear'), a: S('file'), o: S('file'),
  // 字体
  ttf: S('file'), otf: S('file'), woff: S('file'), woff2: S('file'),
}

const SPECIAL_NAME_MAP: Record<string, string> = {
  'dockerfile': B('docker'),
  'makefile': S('gear'), 'rakefile': S('file-code'), 'gemfile': S('file-code'),
  'readme': S('file-lines'), 'license': S('file-lines'), 'licence': S('file-lines'),
  '.gitignore': B('git'), '.gitattributes': B('git'), '.gitmodules': B('git'),
  '.env': S('key'), '.env.local': S('key'), '.env.production': S('key'),
  'package.json': B('node-js'), 'package-lock.json': B('node-js'),
  'go.mod': B('golang'), 'go.sum': B('golang'),
  'cargo.toml': B('rust'), 'cargo.lock': B('rust'),
  'pyproject.toml': B('python'), 'requirements.txt': B('python'), 'pipfile': B('python'),
  'composer.json': B('php'), 'composer.lock': B('php'),
  'tsconfig.json': B('typescript'), 'jsconfig.json': B('js'),
  'vite.config.ts': B('js'), 'vite.config.js': B('js'),
  'webpack.config.js': B('node-js'), 'rollup.config.js': B('node-js'),
  '.eslintrc': S('gear'), '.prettierrc': S('gear'), '.babelrc': S('gear'),
}

function fileIcon(f: { name: string; is_dir?: boolean }): string {
  if (f.is_dir) return '📁'
  const nameLower = f.name.toLowerCase()
  if (SPECIAL_NAME_MAP[nameLower]) return SPECIAL_NAME_MAP[nameLower]
  return FILE_ICON_MAP[fileExt(f.name)] ?? S('file')
}

// ── 行内文件图标（Phosphor 字体图标，按类型区分形状，随主题上色）──
// 仅用于文件列表行；预览弹窗仍用上面的 SVG 版 fileIcon。
const PH_ICON_MAP: Record<string, string> = {
  // 图片
  png: 'ph-file-png', jpg: 'ph-file-jpg', jpeg: 'ph-file-jpg', svg: 'ph-file-svg',
  gif: 'ph-file-image', webp: 'ph-file-image', bmp: 'ph-file-image',
  ico: 'ph-file-image', tiff: 'ph-file-image', tif: 'ph-file-image',
  // 视频 / 音频
  mp4: 'ph-file-video', mkv: 'ph-file-video', avi: 'ph-file-video', mov: 'ph-file-video',
  wmv: 'ph-file-video', flv: 'ph-file-video', webm: 'ph-file-video',
  mp3: 'ph-file-audio', wav: 'ph-file-audio', flac: 'ph-file-audio', aac: 'ph-file-audio',
  ogg: 'ph-file-audio', m4a: 'ph-file-audio', opus: 'ph-file-audio',
  // 压缩包
  zip: 'ph-file-zip', tar: 'ph-file-zip', gz: 'ph-file-zip', bz2: 'ph-file-zip',
  xz: 'ph-file-zip', rar: 'ph-file-zip', '7z': 'ph-file-zip', zst: 'ph-file-zip',
  // 文档
  pdf: 'ph-file-pdf', doc: 'ph-file-doc', docx: 'ph-file-doc',
  xls: 'ph-file-xls', xlsx: 'ph-file-xls', csv: 'ph-file-xls',
  ppt: 'ph-file-ppt', pptx: 'ph-file-ppt',
  // 编程语言
  go: 'ph-file-code', py: 'ph-file-py', pyw: 'ph-file-py', ipynb: 'ph-file-py',
  js: 'ph-file-js', mjs: 'ph-file-js', cjs: 'ph-file-js',
  ts: 'ph-file-ts', mts: 'ph-file-ts', cts: 'ph-file-ts',
  jsx: 'ph-file-jsx', tsx: 'ph-file-tsx', vue: 'ph-file-vue', rs: 'ph-file-rs',
  c: 'ph-file-c', h: 'ph-file-c',
  cpp: 'ph-file-cpp', cc: 'ph-file-cpp', cxx: 'ph-file-cpp', hpp: 'ph-file-cpp', hxx: 'ph-file-cpp',
  cs: 'ph-file-code', java: 'ph-file-code', kt: 'ph-file-code', kts: 'ph-file-code',
  php: 'ph-file-code', swift: 'ph-file-code', dart: 'ph-file-code', rb: 'ph-file-code',
  scala: 'ph-file-code', lua: 'ph-file-code', ex: 'ph-file-code', exs: 'ph-file-code',
  clj: 'ph-file-code', cljs: 'ph-file-code', hs: 'ph-file-code', nim: 'ph-file-code',
  zig: 'ph-file-code', r: 'ph-file-code', rmd: 'ph-file-code', erl: 'ph-file-code',
  // Web
  html: 'ph-file-html', htm: 'ph-file-html',
  css: 'ph-file-css', scss: 'ph-file-css', sass: 'ph-file-css', less: 'ph-file-css', styl: 'ph-file-css',
  // 配置 / 数据
  json: 'ph-gear', jsonc: 'ph-gear', yaml: 'ph-gear', yml: 'ph-gear', toml: 'ph-gear',
  xml: 'ph-gear', plist: 'ph-gear', ini: 'ph-gear', conf: 'ph-gear', cfg: 'ph-gear', config: 'ph-gear',
  env: 'ph-key',
  // 文本 / 文档
  txt: 'ph-file-txt', md: 'ph-file-md', mdx: 'ph-file-md', markdown: 'ph-file-md',
  rst: 'ph-file-txt', adoc: 'ph-file-txt', log: 'ph-file-txt',
  // Shell / 脚本
  sh: 'ph-terminal', bash: 'ph-terminal', zsh: 'ph-terminal', fish: 'ph-terminal',
  ps1: 'ph-terminal', bat: 'ph-terminal', cmd: 'ph-terminal',
  // 数据库
  sql: 'ph-file-sql', db: 'ph-database', sqlite: 'ph-database', sqlite3: 'ph-database',
  // 安全 / 证书
  pem: 'ph-lock', cert: 'ph-lock', crt: 'ph-lock', p12: 'ph-lock', pfx: 'ph-lock',
  key: 'ph-key', pub: 'ph-key',
  // 可执行 / 库
  exe: 'ph-gear', msi: 'ph-gear', so: 'ph-gear', dll: 'ph-gear', dylib: 'ph-gear',
}

const PH_SPECIAL_NAME_MAP: Record<string, string> = {
  dockerfile: 'ph-file-code', makefile: 'ph-gear', rakefile: 'ph-file-code', gemfile: 'ph-file-code',
  readme: 'ph-file-txt', license: 'ph-file-txt', licence: 'ph-file-txt',
  '.gitignore': 'ph-file-code', '.gitattributes': 'ph-file-code', '.gitmodules': 'ph-file-code',
  '.env': 'ph-key', '.env.local': 'ph-key', '.env.production': 'ph-key',
  'package.json': 'ph-file-js', 'package-lock.json': 'ph-file-js',
  'go.mod': 'ph-file-code', 'go.sum': 'ph-file-code',
  'tsconfig.json': 'ph-file-ts', 'jsconfig.json': 'ph-file-js',
}

function fileIconClass(f: { name: string; is_dir?: boolean }): string {
  const nameLower = f.name.toLowerCase()
  return PH_SPECIAL_NAME_MAP[nameLower] ?? PH_ICON_MAP[fileExt(f.name)] ?? 'ph-file'
}

const preview = ref<PreviewState>({
  visible: false, loading: false, error: '',
  name: '', size: 0, type: '', text: '', dataUrl: '',
})

async function loadPreview(f: RemoteFile) {
  const ext  = fileExt(f.name)
  const path = remotePath.value.replace(/\/$/, '') + '/' + f.name
  preview.value = {
    visible: true, loading: true, error: '',
    name: f.name, size: f.size, type: '', text: '', dataUrl: '',
  }
  try {
    if (IMAGE_EXTS.has(ext)) {
      const b64  = await SFTPReadFile(props.connId, path, 4 * 1024 * 1024)
      const mime = ext === 'svg' ? 'image/svg+xml' : `image/${ext === 'jpg' ? 'jpeg' : ext}`
      preview.value.type    = 'image'
      preview.value.dataUrl = `data:${mime};base64,${b64}`
    } else if (TEXT_EXTS.has(ext) || f.size < 512 * 1024) {
      const b64 = await SFTPReadFile(props.connId, path, 256 * 1024)
      preview.value.type = 'text'
      preview.value.text = atob(b64)
    } else {
      preview.value.type = 'binary'
    }
  } catch (e: any) {
    preview.value.error = String(e)
  } finally {
    preview.value.loading = false
  }
}

// ─── Upload ───────────────────────────────────────────────────────────────────

const activeTransferListeners = new Map<string, string[]>()

// ─── Upload ───────────────────────────────────────────────────────────────────

const localFilePickerRef = ref<InstanceType<typeof LocalFilePicker>>()

async function onCtxUpload() {
  ctxMenu.value.visible = false
  const paths = await localFilePickerRef.value?.show()
  if (!paths || paths.length === 0) return
  for (const localPath of paths) {
    const name = localPath.replace(/\\/g, '/').split('/').filter(Boolean).pop() ?? localPath
    const dest = remotePath.value.replace(/\/$/, '') + '/' + name
    try {
      await startUpload(localPath, dest, name)
    } catch (err: any) {
      dlg.error('上传失败：' + String(err))
    }
  }
}

// ─── Download path dialog ─────────────────────────────────────────────────────

interface DlDialogState {
  visible: boolean
  fileName: string
  dirPath: string
  resolve: ((dir: string | null) => void) | null
}

const dlDialog = ref<DlDialogState>({ visible: false, fileName: '', dirPath: '', resolve: null })

/** Show the download path dialog; resolves with dir or null if cancelled. */
async function askDownloadPath(label: string): Promise<string | null> {
  // Get default dir: settings → localStorage fallback → system downloads
  let defaultDir = settingsStore.settings.download_dir
  if (!defaultDir) {
    defaultDir = localStorage.getItem('gssh-download-dir') || ''
  }
  if (!defaultDir) {
    defaultDir = await GetDownloadsDir().catch(() => '')
  }
  return new Promise(resolve => {
    dlDialog.value = { visible: true, fileName: label, dirPath: defaultDir, resolve }
  })
}

function dlDialogConfirm() {
  const dir = dlDialog.value.dirPath.trim()
  dlDialog.value.resolve?.(dir || null)
  dlDialog.value.visible = false
}

function dlDialogCancel() {
  dlDialog.value.resolve?.(null)
  dlDialog.value.visible = false
}

async function browseDlDir() {
  const dir = await OpenDirectoryDialog()
  if (dir) dlDialog.value.dirPath = dir
}

// ─── Download ─────────────────────────────────────────────────────────────────

/** Execute download for a single file to a given dir (no dialog). */
async function doDownloadToDir(f: RemoteFile, dir: string) {
  const src  = remotePath.value.replace(/\/$/, '') + '/' + f.name
  const dest = dir.replace(/\\/g, '/').replace(/\/$/, '') + '/' + f.name
  try {
    await startDownload(src, dest, f.name + (f.is_dir ? '/' : ''), f.is_dir)
  } catch (e: any) {
    dlg.error('下载失败：' + String(e))
  }
}

/** Show download dialog then download a single file. */
async function doDownload(f: RemoteFile) {
  const label = f.name + (f.is_dir ? '/' : '')
  const dir = await askDownloadPath(label)
  if (!dir) return
  await doDownloadToDir(f, dir)
}

/** Show download dialog once, then batch-download all selected files. */
async function startBatchDownload() {
  const files = selectedFiles.value
  if (!files.length) return
  const label = files.length === 1 ? files[0].name : `${files.length} 个项目`
  const dir = await askDownloadPath(label)
  if (!dir) return
  for (const f of files) {
    await doDownloadToDir(f, dir)
  }
}

// ─── Transfer registration ────────────────────────────────────────────────────

function registerTransfer(
  id: string, name: string, direction: 'upload' | 'download' | 'delete',
  retryFn?: () => Promise<void>,
  localPath?: string,
) {
  transfersStore.add(id, props.connId, name, direction, retryFn, localPath)

  const progressKey   = 'sftp:progress:'   + id
  const doneKey       = 'sftp:done:'       + id
  const errKey        = 'sftp:error:'      + id
  const cancelledKey  = 'sftp:cancelled:'  + id

  EventsOn(progressKey, (payload: { done: number; total: number }) => {
    transfersStore.updateProgress(id, payload.done, payload.total)
  })
  EventsOn(doneKey, () => {
    transfersStore.markDone(id)
    cleanupTransfer(id)
    if (direction === 'upload' || direction === 'delete') loadRemote()
  })
  EventsOn(errKey, (msg: string) => {
    transfersStore.markError(id, msg)
    cleanupTransfer(id)
    const verb = direction === 'upload' ? '上传' : direction === 'delete' ? '删除' : '下载'
    dlg.error(verb + '失败：' + msg)
    if (direction === 'delete') loadRemote()   // 部分删除后刷新列表
  })
  EventsOn(cancelledKey, () => {
    transfersStore.markCancelled(id)
    cleanupTransfer(id)
  })

  activeTransferListeners.set(id, [progressKey, doneKey, errKey, cancelledKey])
}

function cleanupTransfer(id: string) {
  const keys = activeTransferListeners.get(id)
  if (keys) { keys.forEach(k => EventsOff(k)); activeTransferListeners.delete(id) }
}

/** 启动单文件/目录上传，并注入重试闭包。 */
async function startUpload(localPath: string, dest: string, name: string) {
  const doUpload = async () => {
    const isDir = await LocalIsDir(localPath)
    const tid   = isDir
      ? await SFTPUploadDir(props.connId, localPath, dest)
      : await SFTPUpload(props.connId, localPath, dest)
    registerTransfer(tid, name, 'upload', doUpload)
  }
  await doUpload()
}

/** 启动单文件/目录下载，并注入重试闭包。 */
async function startDownload(remoteSrc: string, localDest: string, name: string, isDir: boolean) {
  const doDownload = async () => {
    const tid = isDir
      ? await SFTPDownloadDir(props.connId, remoteSrc, localDest)
      : await SFTPDownload(props.connId, remoteSrc, localDest)
    registerTransfer(tid, name, 'download', doDownload, localDest)
  }
  await doDownload()
}

/** 启动删除（一批远程路径），进度显示在底部传输框；注入重试闭包。 */
async function startDelete(paths: string[], name: string) {
  const doDelete = async () => {
    const tid = await SFTPDeleteWithProgress(props.connId, paths)
    registerTransfer(tid, name, 'delete', doDelete)
  }
  await doDelete()
}

// ─── Context menu ─────────────────────────────────────────────────────────────

const ctxMenu = ref({ visible: false, x: 0, y: 0 })

function openCtxMenu(e: MouseEvent, f: RemoteFile | null) {
  // Right-click on empty area → clear selection
  if (!f) clearSelection()
  // Right-click on unselected file → select it first
  else if (!selectedNames.value.has(f.name)) {
    selectedNames.value = new Set([f.name])
    anchorName.value = f.name
  }
  ctxMenu.value = { visible: true, x: e.clientX, y: e.clientY }
  // Overflow fix: adjust position after render
  nextTick(() => {
    const el = document.querySelector<HTMLElement>('.ctx-menu')
    if (!el) return
    const rect = el.getBoundingClientRect()
    let x = e.clientX, y = e.clientY
    if (x + rect.width  > window.innerWidth)  x = window.innerWidth  - rect.width  - 4
    if (y + rect.height > window.innerHeight) y = window.innerHeight - rect.height - 4
    ctxMenu.value.x = Math.max(4, x)
    ctxMenu.value.y = Math.max(4, y)
  })
}

function onCtxPreview() {
  const f = ctxSingleFile.value; ctxMenu.value.visible = false
  if (!f || f.is_dir) return
  loadPreview(f)
}

async function onCtxDownload() {
  const f = ctxSingleFile.value; ctxMenu.value.visible = false
  if (f) await doDownload(f)
}

async function onCtxBatchDownload() {
  ctxMenu.value.visible = false
  await startBatchDownload()
}

async function onCtxEdit() {
  const f = ctxSingleFile.value; ctxMenu.value.visible = false
  if (!f || f.is_dir) return
  await openWithEditor(f)
}

async function onCtxRename() {
  const f = ctxSingleFile.value; ctxMenu.value.visible = false
  if (!f) return
  await doRename(f)
}

// ─── 重命名（弹窗形式） ───────────────────────────────────────────────────────

async function doRename(f: RemoteFile) {
  // 文件：仅选中名称（不含扩展名）；目录：全选
  const selectMode = f.is_dir ? 'all' : 'name'
  const newName = await dlg.prompt('输入新名称', '重命名', '', f.name, selectMode)
  if (!newName || newName === f.name) return
  const oldPath = remotePath.value.replace(/\/$/, '') + '/' + f.name
  const newPath = remotePath.value.replace(/\/$/, '') + '/' + newName
  try {
    await SFTPRename(props.connId, oldPath, newPath)
    await loadRemote()
  } catch (e: any) { dlg.error('重命名失败：' + String(e)) }
}

// ─── 移动（重命名到目标目录） ─────────────────────────────────────────────────

async function onCtxMove() {
  const f = ctxSingleFile.value; ctxMenu.value.visible = false
  if (!f) return
  await moveItems([f.name])
}

async function onCtxBatchMove() {
  ctxMenu.value.visible = false
  await moveItems([...selectedNames.value])
}

/** 将一批文件/目录移动到目标目录（同一 SFTP 上为即时 rename）。 */
async function moveItems(names: string[]) {
  if (names.length === 0) return
  const base  = remotePath.value.replace(/\/$/, '')
  const title = names.length === 1 ? `移动 "${names[0]}"` : `移动 ${names.length} 项`
  const dest  = await dlg.prompt('输入目标目录的绝对路径', title, '/path/to/dir', base, 'all')
  if (dest === null) return
  const target = dest.trim().replace(/\/+$/, '') || '/'
  if (target === base) return   // 同目录，无需移动
  const errors: string[] = []
  for (const name of names) {
    try {
      await SFTPRename(props.connId, base + '/' + name, target + '/' + name)
    } catch (e: any) { errors.push(`${name}: ${String(e)}`) }
  }
  if (errors.length) dlg.error('部分移动失败：\n' + errors.join('\n'))
  clearSelection()
  await loadRemote()
}

async function onCtxCopyPath() {
  const f = ctxSingleFile.value; ctxMenu.value.visible = false
  if (!f) return
  const full = remotePath.value.replace(/\/$/, '') + '/' + f.name
  try {
    await navigator.clipboard.writeText(full)
  } catch {
    dlg.error('复制到剪贴板失败')
  }
}

// ─── Chmod ────────────────────────────────────────────────────────────────────

const chmodDialog = ref({ visible: false, name: '', path: '' })
const chmodSaving = ref(false)
// chmodBits[0]=owner [1]=group [2]=other, each [r,w,x]
const chmodBits = ref([[true,true,false],[true,false,false],[true,false,false]])

const chmodOctal = computed({
  get() {
    return chmodBits.value.map(row =>
      (row[0] ? 4 : 0) + (row[1] ? 2 : 0) + (row[2] ? 1 : 0)
    ).join('')
  },
  set(v: string) { onChmodOctalInput(v) },
})

const chmodRows = computed(() =>
  [['所有者', chmodBits.value[0]], ['所属组', chmodBits.value[1]], ['其他人', chmodBits.value[2]]]
    .map(([label, bits]) => ({ label, bits }))
)

function onChmodBit(row: number, col: number, val: boolean) {
  chmodBits.value[row][col] = val
}

function onChmodOctalInput(v: string | Event) {
  const s = typeof v === 'string' ? v : (v.target as HTMLInputElement).value
  if (!/^[0-7]{1,4}$/.test(s)) return
  const num = parseInt(s.slice(-3).padStart(3, '0'), 8)
  chmodBits.value = [
    [!!(num & 0o400), !!(num & 0o200), !!(num & 0o100)],
    [!!(num & 0o040), !!(num & 0o020), !!(num & 0o010)],
    [!!(num & 0o004), !!(num & 0o002), !!(num & 0o001)],
  ]
}

async function onCtxChmod() {
  const f = ctxSingleFile.value; ctxMenu.value.visible = false
  if (!f) return
  const full = remotePath.value.replace(/\/$/, '') + '/' + f.name
  try {
    const info = await SFTPStat(props.connId, full) as { mode: string }
    const mode = parseInt((info.mode || '644').replace(/[^0-7]/g, '').slice(-3), 8)
    onChmodOctalInput(mode.toString(8).padStart(3, '0'))
  } catch { /* use defaults */ }
  chmodDialog.value = { visible: true, name: f.name, path: full }
}

async function commitChmod() {
  chmodSaving.value = true
  const octal = parseInt(chmodOctal.value, 8)
  try {
    await SFTPChmod(props.connId, chmodDialog.value.path, octal)
    chmodDialog.value.visible = false
    await loadRemote()
  } catch (e: any) {
    dlg.error('修改权限失败：' + String(e))
  } finally {
    chmodSaving.value = false
  }
}

async function onCtxDelete() {
  const f = ctxSingleFile.value; ctxMenu.value.visible = false
  if (!f) return
  const ok = await dlg.confirm(`确定要删除 "${f.name}"？\n此操作不可恢复。`, '删除确认')
  if (!ok) return
  const p = remotePath.value.replace(/\/$/, '') + '/' + f.name
  await startDelete([p], f.name)
}

async function onCtxBatchDelete() {
  ctxMenu.value.visible = false
  await batchDelete()
}

function onCtxMkdir() {
  ctxMenu.value.visible = false
  mkdirPrompt()
}

function onCtxNewFile() {
  ctxMenu.value.visible = false
  mkfilePrompt()
}

function onCtxRefresh() {
  ctxMenu.value.visible = false
  loadRemote()
}

// ─── Mkdir / Create file ─────────────────────────────────────────────────────

async function mkdirPrompt() {
  const name = await dlg.prompt('请输入新目录名称', '新建目录', 'folder-name')
  if (!name) return
  const newPath = remotePath.value.replace(/\/$/, '') + '/' + name
  try {
    await SFTPMkdir(props.connId, newPath)
    await loadRemote()
  } catch (e: any) {
    dlg.error('创建失败：' + String(e))
  }
}

async function mkfilePrompt() {
  const name = await dlg.prompt('请输入新文件名称', '新建文件', 'newfile.txt')
  if (!name) return
  const newPath = remotePath.value.replace(/\/$/, '') + '/' + name
  try {
    await SFTPCreateFile(props.connId, newPath)
    await loadRemote()
  } catch (e: any) {
    dlg.error('创建失败：' + String(e))
  }
}

// ─── Batch delete ─────────────────────────────────────────────────────────────

async function batchDelete() {
  const n = selectedNames.value.size
  if (n === 0) return
  const names = [...selectedNames.value]
  const label = n === 1 ? `"${names[0]}"` : `选中的 ${n} 项`
  const ok = await dlg.confirm(`确定要删除 ${label}？\n此操作不可恢复。`, '删除确认')
  if (!ok) return
  const base   = remotePath.value.replace(/\/$/, '')
  const paths  = names.map(name => base + '/' + name)
  const dispName = n === 1 ? names[0] : `${n} 项`
  clearSelection()
  await startDelete(paths, dispName)
}

// ─── Local editor integration ─────────────────────────────────────────────────

const openingEditor = ref(false)
const editorPrompt  = ref({ visible: false, watchID: '', name: '', connID: '' })
const activeEditorWatchers = new Set<string>()

async function openWithEditor(f: RemoteFile) {
  if (f.is_dir || openingEditor.value) return
  openingEditor.value = true
  try {
    const remoteFull = remotePath.value.replace(/\/$/, '') + '/' + f.name
    const watchID = await OpenWithEditor(props.connId, remoteFull)
    activeEditorWatchers.add(watchID)
    EventsOn(`editor:changed:${watchID}`, () => {
      editorPrompt.value = { visible: true, watchID, name: f.name, connID: props.connId }
    })
  } catch (e: any) {
    dlg.error('打开编辑器失败：' + String(e))
  } finally {
    openingEditor.value = false
  }
}

async function confirmEditorUpload() {
  const { watchID, connID, name } = editorPrompt.value
  editorPrompt.value.visible = false
  try {
    await UploadEditedFile(connID, watchID)
    loadRemote()
  } catch (e: any) {
    if (/permission denied/i.test(String(e))) {
      await confirmEditorUploadSudo(connID, watchID, name)
    } else {
      dlg.error('上传失败：' + String(e))
    }
  }
}

// 权限不足时的 sudo 提权保存：优先复用后端缓存的密码（首次为空会返回
// "sudo password required"）；需要密码或认证失败时询问，其他错误直接报错。
async function confirmEditorUploadSudo(connID: string, watchID: string, name: string) {
  const ok = await dlg.confirm(
    `保存「${name}」失败：目标文件没有写入权限。\n是否使用 sudo 提权保存？`,
    '权限不足',
  )
  if (!ok) return
  let pwd = ''
  for (;;) {
    try {
      await UploadEditedFileSudo(connID, watchID, pwd)
      loadRemote()
      return
    } catch (e: any) {
      const msg = String(e)
      if (msg.includes('sudo password required') || msg.includes('sudo auth failed')) {
        const promptMsg = msg.includes('sudo auth failed')
          ? 'sudo 密码错误，请重新输入'
          : `请输入 sudo 密码以保存「${name}」`
        const input = await dlg.password(promptMsg, 'sudo 提权')
        if (input === null) return
        pwd = input
        continue
      }
      dlg.error('sudo 保存失败：' + msg)
      return
    }
  }
}

function dismissEditorPrompt() { editorPrompt.value.visible = false }

function stopEditorWatch() {
  const { watchID } = editorPrompt.value
  editorPrompt.value.visible = false
  EventsOff(`editor:changed:${watchID}`)
  StopEditorWatch(watchID)
  activeEditorWatchers.delete(watchID)
}

// ─── Column resize ───────────────────────────────────────────────────────────

const COL_WIDTHS_KEY = 'gssh-fm-col-widths'

const COL_DEFAULT_WIDTHS: Record<FMColumn, number> = {
  name: 220, mode: 90, owner: 100, group: 100, size: 80, mod_time: 170,
}

function loadColWidths(): Record<FMColumn, number> {
  try {
    const saved = JSON.parse(localStorage.getItem(COL_WIDTHS_KEY) ?? '{}')
    return { ...COL_DEFAULT_WIDTHS, ...saved }
  } catch {
    return { ...COL_DEFAULT_WIDTHS }
  }
}

const colWidths  = ref<Record<FMColumn, number>>(loadColWidths())
const isResizing = ref(false)

// Plain variables — only updated during drag, no reactivity needed
let resizingCol: FMColumn | null = null
let resizeStartX = 0
let resizeStartW = 0

function onColResizeStart(e: MouseEvent, col: FMColumn) {
  resizingCol  = col
  resizeStartX = e.clientX
  resizeStartW = colWidths.value[col]
  isResizing.value = true
  document.addEventListener('mousemove', onColResizeMove)
  document.addEventListener('mouseup',   onColResizeEnd)
}

function onColResizeMove(e: MouseEvent) {
  if (!resizingCol) return
  colWidths.value[resizingCol] = Math.max(50, resizeStartW + (e.clientX - resizeStartX))
}

function onColResizeEnd() {
  document.removeEventListener('mousemove', onColResizeMove)
  document.removeEventListener('mouseup',   onColResizeEnd)
  isResizing.value = false
  if (resizingCol) {
    localStorage.setItem(COL_WIDTHS_KEY, JSON.stringify(colWidths.value))
    resizingCol = null
  }
}

function resetColWidth(col: FMColumn) {
  colWidths.value[col] = COL_DEFAULT_WIDTHS[col]
  localStorage.setItem(COL_WIDTHS_KEY, JSON.stringify(colWidths.value))
}

// ─── Drag selection (rubber-band) ────────────────────────────────────────────

const dragSel = reactive({
  active: false,
  startX: 0,
  startY: 0,
  endX: 0,
  endY: 0,
})

const dragSelStyle = computed(() => {
  const left   = Math.min(dragSel.startX, dragSel.endX)
  const top    = Math.min(dragSel.startY, dragSel.endY)
  const width  = Math.abs(dragSel.endX - dragSel.startX)
  const height = Math.abs(dragSel.endY - dragSel.startY)
  return { left: left + 'px', top: top + 'px', width: width + 'px', height: height + 'px' }
})

// Plain vars for drag state (no reactivity needed mid-drag)
let _dragStarted = false
let _dragOriginX = 0
let _dragOriginY = 0
let _hadDrag     = false   // prevents @click.self from clearing selection after drag

/** Click on empty list area — only clear selection if no drag just happened */
function onListClickSelf() {
  if (_hadDrag) { _hadDrag = false; return }
  clearSelection()
}

function onListMousedown(e: MouseEvent) {
  // Only left button
  if (e.button !== 0) return
  // Ignore column-resize handle clicks
  if ((e.target as Element).closest('.col-resize-handle')) return

  // Don't preventDefault here — let row @click handlers still fire on simple clicks.
  // Text selection suppression is handled in onDragMove once threshold is crossed.

  _dragStarted = false
  _dragOriginX = e.clientX
  _dragOriginY = e.clientY

  document.addEventListener('mousemove', onDragMove)
  document.addEventListener('mouseup',   onDragEnd)
}

function onDragMove(e: MouseEvent) {
  const dx = e.clientX - _dragOriginX
  const dy = e.clientY - _dragOriginY

  if (!_dragStarted) {
    // 5px threshold before rubber-band starts
    if (Math.sqrt(dx * dx + dy * dy) < 5) return
    _dragStarted = true
    clearSelection()
    dragSel.active = true
    dragSel.startX = _dragOriginX
    dragSel.startY = _dragOriginY
    // Suppress browser text-selection for the duration of the drag
    document.body.style.userSelect = 'none'
    window.getSelection()?.removeAllRanges()
  }

  dragSel.endX = e.clientX
  dragSel.endY = e.clientY
  updateDragSelection()
}

function onDragEnd() {
  document.removeEventListener('mousemove', onDragMove)
  document.removeEventListener('mouseup',   onDragEnd)
  // Always restore text-selection (even if drag didn't cross threshold)
  document.body.style.userSelect = ''
  if (_dragStarted) {
    dragSel.active = false
    _dragStarted = false
    // Mark that a drag just happened:
    // 1. @click.self (onListClickSelf) won't clear selection
    // 2. @click on row (onRowClick) won't override drag selection
    _hadDrag = true
    setTimeout(() => { _hadDrag = false }, 100)
  }
}

function updateDragSelection() {
  if (!listBodyRef.value) return
  const selLeft   = Math.min(dragSel.startX, dragSel.endX)
  const selTop    = Math.min(dragSel.startY, dragSel.endY)
  const selRight  = Math.max(dragSel.startX, dragSel.endX)
  const selBottom = Math.max(dragSel.startY, dragSel.endY)

  const rows = Array.from(listBodyRef.value.querySelectorAll<HTMLTableRowElement>('tr[data-idx]'))
  const next = new Set<string>()

  for (const row of rows) {
    const r = row.getBoundingClientRect()
    // Intersect row rect with drag rect
    if (r.bottom > selTop && r.top < selBottom && r.right > selLeft && r.left < selRight) {
      const idx = parseInt(row.dataset.idx ?? '-1')
      if (idx >= 0 && filteredFiles.value[idx]) {
        next.add(filteredFiles.value[idx].name)
      }
    }
  }

  selectedNames.value = next
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function fmtTime(t: string): string {
  if (!t) return ''
  try {
    return new Date(t).toLocaleString('zh-CN', {
      year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit', second: '2-digit',
      hour12: false,
    })
  } catch { return t }
}

function fmtSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB'
  return (bytes / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}


// ─── Lifecycle ────────────────────────────────────────────────────────────────

// 拖拽处理：立即清掉选中态给出反馈，然后开始上传
async function handleFileDrop(_x: number, _y: number, paths: string[]) {
  // 仅活动 tab 的 FileManager 才处理（防御性二次检查）
  if (tabsStore.activeConnId !== props.connId) return

  // DisableWebViewDrop:true 模式下 dragover DOM 事件不触发，remoteDropActive 无法
  // 在悬停阶段由 @dragover 置为 true。改为在 drop 发生时短暂高亮，确认"已接收"。
  remoteDropActive.value = true
  setTimeout(() => { remoteDropActive.value = false }, 600)

  selectedNames.value = new Set()
  anchorName.value    = null
  focusedIdx.value    = -1

  for (const localPath of paths) {
    const name = localPath.replace(/\\/g, '/').split('/').pop() || localPath
    const dest = remotePath.value.replace(/\/$/, '') + '/' + name
    try {
      await startUpload(localPath, dest, name)
    } catch (err: any) {
      dlg.error('上传失败：' + String(err))
    }
  }
}

// Wails 的 OnFileDrop 是全局单例：内部有 flags.registered 守卫，第二次调用直接
// return，不会替换回调。因此每次（重新）注册前必须先调 OnFileDropOff() 重置该
// 标志，再调 OnFileDrop() 注册本 tab 的回调。
// onUnmounted 中保留原有守卫：由于 Vue pre-flush watch 先于 onUnmounted 执行，
// 切换 tab 时新 tab 的 watch 已完成注册，onUnmounted 的守卫可防止它再清掉新
// tab 的回调。
watch(() => tabsStore.activeConnId, (newId) => {
  if (newId === props.connId) {
    OnFileDropOff()                  // 重置单例，清除上一个 tab 残留的注册
    OnFileDrop(handleFileDrop, true)
  }
})

let _listResizeObserver: ResizeObserver | null = null

onMounted(async () => {
  // onMounted 时确保初始注册（watch immediate 因组件 mount 顺序不可靠）
  if (tabsStore.activeConnId === props.connId) {
    OnFileDropOff()                  // 清除上次关闭的 tab 可能残留的注册
    OnFileDrop(handleFileDrop, true)
  }

  // Init virtual scroll viewport height and keep it in sync
  if (listBodyRef.value) {
    listClientHeight.value = listBodyRef.value.clientHeight
    _listResizeObserver = new ResizeObserver(() => {
      if (listBodyRef.value) listClientHeight.value = listBodyRef.value.clientHeight
    })
    _listResizeObserver.observe(listBodyRef.value)
  }

  try {
    const home = await SFTPHomePath(props.connId)
    await navigateTo(home || '/')
  } catch {
    await navigateTo('/')
  }
})

onUnmounted(() => {
  // 只有自己仍是活动 tab 时才清回调（不影响其它 FileManager）
  if (tabsStore.activeConnId === props.connId) {
    OnFileDropOff()
  }
  _listResizeObserver?.disconnect()
  for (const [id] of activeTransferListeners) cleanupTransfer(id)
  for (const watchID of activeEditorWatchers) {
    EventsOff(`editor:changed:${watchID}`)
    StopEditorWatch(watchID)
  }
  // Clean up any in-progress column resize listeners
  document.removeEventListener('mousemove', onColResizeMove)
  document.removeEventListener('mouseup',   onColResizeEnd)
  // Clean up drag selection listeners
  document.removeEventListener('mousemove', onDragMove)
  document.removeEventListener('mouseup',   onDragEnd)
  // Clean up any in-progress column-header drag
  _colDragCleanup?.()
})
</script>

<style scoped>
.fm-root {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--panel-2);
  color: var(--tp);
  font-size: 12px;
  overflow: hidden;
  font-family: 'IBM Plex Sans', 'Segoe UI', system-ui, sans-serif;
  -webkit-font-smoothing: antialiased;
}
.mono { font-family: 'IBM Plex Mono', 'Roboto Mono', monospace; }
.fm-spring { flex: 1; }

/* 面板头 */
.fm-head {
  height: 38px;
  flex: 0 0 38px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 14px;
  border-bottom: 1px solid var(--line-soft);
}
.fm-head-icon { font-size: 15px; color: var(--accent); }
.fm-head-title { font-size: 11px; font-weight: 600; letter-spacing: 1.2px; color: var(--ts); text-transform: uppercase; }
.fm-sftp-tag { font-size: 10px; color: var(--tm); background: var(--hover); padding: 2px 7px; border-radius: 5px; }

/* drop-active: triggered by @dragover (HTML5 drag) or handleFileDrop flash */
/* wails-drop-target-active: added by Wails runtime when dragover DOM event fires */
.fm-root.drop-active,
.fm-root.wails-drop-target-active {
  outline: 2px dashed var(--accent);
  outline-offset: -2px;
  background: color-mix(in srgb, var(--accent) 5%, var(--bg-primary));
  transition: background 0.1s;
}

/* ── 工具栏（两行） ── */
.fm-toolbar {
  display: flex;
  flex-direction: column;
  background: var(--panel-2);
  flex-shrink: 0;
}

/* 第一行：路径 + 导航 */
.fm-nav-row {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 10px 12px 8px;
}

/* 第二行：过滤 */
.fm-filter-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px 10px;
}

/* 路径框（house 图标 + 输入） */
.fm-path-box {
  flex: 1;
  min-width: 0;
  height: 32px;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 7px;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 10px;
  transition: border-color 0.15s;
}
.fm-path-box:focus-within { border-color: var(--accent); }
.fm-path-icon { font-size: 13px; color: var(--tm); flex-shrink: 0; }
.fm-path-input {
  flex: 1;
  min-width: 0;
  background: transparent;
  border: none;
  outline: none;
  padding: 0;
  color: var(--tp);
  font-size: 12.5px;
  cursor: pointer;
}
.fm-path-input:focus { cursor: text; }

/* 导航按钮 32×32 */
.fm-nav-btn {
  width: 32px;
  height: 32px;
  flex: 0 0 32px;
  border-radius: 7px;
  background: var(--panel);
  border: 1px solid var(--line);
  color: var(--ts);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 15px;
  transition: color 0.15s, background 0.15s;
}
.fm-nav-btn:disabled { opacity: 0.3; cursor: not-allowed; }
.fm-nav-btn:not(:disabled):hover { color: var(--tp); background: var(--hover); }

/* 过滤框 */
.fm-filter-box {
  flex: 1;
  min-width: 0;
  height: 30px;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 7px;
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 0 10px;
  transition: border-color 0.15s;
}
.fm-filter-box:focus-within { border-color: var(--accent); }
.fm-filter-icon { font-size: 13px; color: var(--tm); flex-shrink: 0; }
.fm-filter-input {
  flex: 1;
  min-width: 0;
  background: transparent;
  border: none;
  outline: none;
  padding: 0;
  color: var(--tp);
  font-size: 12px;
}
.fm-filter-input::placeholder { color: var(--tm); }

/* 隐藏文件拨杆开关 */
.fm-hidden-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  user-select: none;
  flex-shrink: 0;
}
.fm-hidden-toggle:hover { opacity: 0.85; }
.fm-switch {
  width: 30px;
  height: 17px;
  border-radius: 9px;
  background: var(--hover);
  position: relative;
  transition: background 0.15s;
  flex-shrink: 0;
}
.fm-switch-knob {
  width: 13px;
  height: 13px;
  border-radius: 50%;
  background: var(--tm);
  position: absolute;
  top: 2px;
  left: 2px;
  transition: left 0.15s, background 0.15s;
}
.fm-hidden-toggle.active .fm-switch { background: var(--accent-soft); }
.fm-hidden-toggle.active .fm-switch-knob { left: 15px; background: var(--accent); }
.fm-switch-label { font-size: 11.5px; color: var(--ts); white-space: nowrap; }

/* ── 主体 ── */
.fm-body {
  display: flex;
  flex: 1;
  overflow: hidden;
  background: var(--panel-2);
}

/* 文件行图标 */
.fm-icon-folder { color: var(--gold, #d3a75a); font-size: 15px; }
.fm-icon-file { color: var(--ts); font-size: 15px; }

/* 底栏拖拽提示图标 */
.fm-drop-hint { display: inline-flex; align-items: center; gap: 4px; }

/* ── 文件列表 ── */
.fm-list {
  flex: 1;
  overflow: auto;
  min-width: 0;
  outline: none;
}

.fm-list:focus-within { box-shadow: inset 0 0 0 1px rgba(88,166,255,0.15); }

.fm-loading,
.fm-error {
  padding: 20px;
  text-align: center;
  color: var(--text-muted);
}

.fm-error { color: var(--danger); }

.fm-table {
  width: 100%;
  min-width: 360px;
  border-collapse: collapse;
  font-size: 13px;
  table-layout: fixed;
}

.fm-table thead tr {
  background: var(--panel-2);
  position: sticky;
  top: 0;
  z-index: 1;
}

.fm-table th {
  padding: 6px 10px;
  text-align: left;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.5px;
  color: var(--ts);
  white-space: nowrap;
  border-top: 1px solid var(--line-soft);
  border-bottom: 1px solid var(--line-soft);
  user-select: none;
  position: relative;
  overflow: hidden;
}

.col-sortable {
  cursor: pointer;
  transition: color 0.15s, background 0.15s;
  border-right: 1px solid var(--line-soft);   /* 列与列之间的分隔线 */
}
.col-sortable:hover { color: var(--tp); background: var(--hover); }
.sort-active { color: var(--accent) !important; }

/* 列头拖拽重排 —— 被拖的列 */
.col-sortable.col-dragging {
  opacity: 0.35;
  background: rgba(88,166,255,0.12);
}
/* 落点列：整列高亮 + 左侧一条醒目的 accent 竖线 */
.col-sortable.col-drag-over { background: rgba(88,166,255,0.12); }
.col-sortable.col-drag-over::before {
  content: '';
  position: absolute;
  left: -1px; top: 0; bottom: 0;
  width: 3px;
  background: var(--accent);
  box-shadow: 0 0 8px var(--accent);
  z-index: 2;
}

/* 拖拽时跟随光标的浮动标签 */
.col-drag-ghost {
  position: fixed;
  z-index: 200;
  pointer-events: none;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 11px;
  font-size: 12px;
  font-weight: 600;
  color: #fff;
  background: var(--accent);
  border-radius: 6px;
  box-shadow: 0 8px 20px rgba(0,0,0,0.45);
  white-space: nowrap;
}
.col-drag-ghost-grip { opacity: 0.75; font-size: 10px; letter-spacing: -1px; }

.sort-arrow { display: inline-block; width: 12px; font-size: 11px; }

/* 列标题文字 */
.col-label { display: inline; }

/* 拖拽调宽 handle */
.col-resize-handle {
  position: absolute;
  right: 0;
  top: 0;
  bottom: 0;
  width: 6px;
  cursor: col-resize;
  z-index: 1;
}
.col-resize-handle:hover,
.col-resize-handle:active {
  background: var(--accent);
  opacity: 0.5;
}

/* 拖拽时全屏覆盖层，固定光标不跳变 */
.col-resize-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  cursor: col-resize;
}

/* 框选矩形 */
.drag-sel-box {
  position: fixed;
  border: 1px solid var(--accent);
  background: rgba(88, 166, 255, 0.08);
  pointer-events: none;
  z-index: 150;
  border-radius: 2px;
}

/* spacer 列 */
.col-spacer { padding: 0; border: none; }

.fm-table tbody tr {
  cursor: pointer;
  transition: background 0.08s;
}

/* 虚拟滚动占位行：无交互、无边框 */
.fm-table tbody tr.fm-vspacer {
  cursor: default;
  transition: none;
  pointer-events: none;
}
.fm-table tbody tr.fm-vspacer td {
  padding: 0;
  border: none;
}

.fm-table tbody tr:hover                         { background: var(--hover); }
.fm-table tbody tr.selected                      { background: var(--accent-soft); box-shadow: inset 2.5px 0 0 var(--accent); }
.fm-table tbody tr.focused:not(.selected)        { background: var(--hover); box-shadow: inset 0 0 0 1px var(--accent-soft); }
.fm-table tbody tr.selected.focused              { box-shadow: inset 2.5px 0 0 var(--accent), inset 0 0 0 1px var(--accent-soft); }

.fm-table td {
  padding: 0 10px;
  height: 30px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  border-bottom: none;
}

td.col-icon { width: 30px; padding: 0 0 0 10px; text-align: left; line-height: 1; }
.file-type-icon {
  width: 14px;
  height: 14px;
  display: inline-block;
  vertical-align: middle;
  filter: brightness(1.1);
}
.file-type-emoji {
  font-size: 14px;
  line-height: 1;
  vertical-align: middle;
}
td.fm-col-name {
  font-size: 12.5px;
  padding-left: 9px;
  color: var(--tp);
  font-family: 'IBM Plex Mono', 'Roboto Mono', monospace;
}
td.fm-col-size,
td.fm-col-owner,
td.fm-col-group,
td.fm-col-mode,
td.fm-col-mod_time {
  color: var(--tm);
  font-size: 11px;
  font-family: 'IBM Plex Mono', 'Roboto Mono', monospace;
}

.fm-empty-cell {
  padding: 24px;
  text-align: center;
  color: var(--text-muted);
  font-style: italic;
  font-size: 12px;
}

/* ── 底栏（状态 + 拖拽提示） ── */
.fm-footer {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 14px;
  height: 30px;
  flex: 0 0 30px;
  background: var(--panel);
  border-top: 1px solid var(--line);
  font-size: 11px;
  color: var(--tm);
}

.fm-status-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fm-drop-hint {
  flex-shrink: 0;
  color: var(--tm);
  opacity: 0.6;
  pointer-events: none;
  user-select: none;
}
.fm-drop-hint i { font-size: 12px; }

.fm-btn {
  padding: 3px 10px;
  border-radius: 5px;
  border: 1px solid var(--border);
  cursor: pointer;
  font-size: 11px;
  background: transparent;
  color: var(--text-primary);
  white-space: nowrap;
  flex-shrink: 0;
  transition: background 0.15s;
}

.fm-btn:disabled { opacity: 0.4; cursor: not-allowed; }

.fm-btn-primary {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}

.fm-btn-primary:hover:not(:disabled) { filter: brightness(1.1); }

/* ── 右键菜单 ── */
.ctx-overlay { position: fixed; inset: 0; z-index: 299; }

.ctx-menu {
  position: fixed;
  z-index: 300;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 7px;
  padding: 4px;
  min-width: 168px;
  box-shadow: 0 8px 24px rgba(0,0,0,0.5);
}

.ctx-item {
  padding: 7px 12px;
  font-size: 12px;
  color: var(--text-primary);
  border-radius: 4px;
  cursor: pointer;
}

.ctx-item:hover { background: rgba(255,255,255,0.06); }
.ctx-item.danger { color: var(--danger); }
.ctx-item.danger:hover { background: rgba(248,81,73,0.1); }

.ctx-divider { height: 1px; background: var(--border); margin: 3px 4px; }


/* ── 下载路径对话框 ── */
.dl-overlay {
  position: absolute;
  inset: 0;
  z-index: 400;
  background: rgba(0,0,0,0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  backdrop-filter: blur(2px);
}

.dl-dialog {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 10px;
  width: 360px;
  max-width: 90%;
  box-shadow: 0 12px 40px rgba(0,0,0,0.6);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.dl-dialog-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 14px 10px;
  border-bottom: 1px solid var(--border);
}

.dl-dialog-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}

.dl-close-btn {
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 12px;
  padding: 2px 5px;
  border-radius: 3px;
}
.dl-close-btn:hover { color: var(--text-primary); background: rgba(255,255,255,0.06); }

.dl-dialog-body {
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.dl-file-name {
  font-size: 12px;
  color: var(--text-primary);
  background: rgba(255,255,255,0.04);
  border: 1px solid var(--border);
  border-radius: 5px;
  padding: 6px 10px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: 'Cascadia Code', Consolas, monospace;
}

.dl-path-row {
  display: flex;
  gap: 6px;
  align-items: center;
}

.dl-path-input {
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
.dl-path-input:focus { border-color: var(--accent); }
.dl-path-input::placeholder { color: var(--text-muted); opacity: 0.6; }

.dl-dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 10px 14px;
  border-top: 1px solid var(--border);
}

/* ── Chmod 弹窗 ── */
.chmod-dialog { min-width: 280px; max-width: 340px; }

.chmod-matrix {
  display: grid;
  grid-template-columns: 60px 1fr 1fr 1fr;
  gap: 4px 0;
  margin: 10px 0;
  font-size: 12px;
}
.chmod-row-header { display: contents; font-weight: 600; color: var(--text-muted); }
.chmod-row-header span, .chmod-row-header span { text-align: center; padding-bottom: 4px; }
.chmod-row { display: contents; }
.chmod-label { color: var(--text-muted); line-height: 28px; }
.chmod-cb { display: flex; align-items: center; justify-content: center; height: 28px; cursor: pointer; }
.chmod-cb input { width: 14px; height: 14px; accent-color: var(--accent); cursor: pointer; }

.chmod-octal-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
  border-top: 1px solid var(--border);
  padding-top: 8px;
}
.chmod-octal-label { font-size: 12px; color: var(--text-muted); }
.chmod-octal-input {
  width: 60px;
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-primary);
  font-size: 13px;
  font-family: monospace;
  padding: 3px 6px;
  text-align: center;
}
.chmod-octal-input:focus { outline: none; border-color: var(--accent); }

/* ── 编辑器提示 ── */
.editor-prompt-overlay {
  position: absolute;
  inset: 0;
  z-index: 400;
  background: rgba(0,0,0,0.5);
  display: flex;
  align-items: center;
  justify-content: center;
}

.editor-prompt {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 18px 20px;
  max-width: 300px;
  width: 90%;
  box-shadow: 0 12px 40px rgba(0,0,0,0.6);
}

.ep-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 8px;
}

.ep-body {
  font-size: 12px;
  color: var(--text-muted);
  margin-bottom: 14px;
  line-height: 1.6;
}

.ep-body strong { color: var(--text-primary); }

.ep-actions {
  display: flex;
  gap: 6px;
  align-items: center;
}
</style>
