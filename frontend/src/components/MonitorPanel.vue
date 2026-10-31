<template>
  <div class="mp">
    <!-- 头部 -->
    <div class="mp-head">
      <i class="ph-fill ph-pulse mp-head-icon"></i>
      <span class="mp-head-title">服务器监控</span>
      <span class="mp-breath"></span>
      <div class="mp-spring"></div>
      <span class="mp-host mono">{{ hostLabel }}</span>
    </div>

    <div v-if="stats.error" class="mp-error">{{ stats.error }}</div>

    <template v-else>
      <!-- CPU -->
      <div class="mp-cpu">
        <div class="mp-row">
          <span class="mp-label">CPU</span>
          <span v-if="stats.cpu_temp >= 0" class="mp-temp mono" :class="{ hot: stats.cpu_temp >= 80 }">{{ stats.cpu_temp.toFixed(1) }}°C</span>
          <div class="mp-spring"></div>
          <span class="mp-big mono" :style="{ color: loadColor(stats.cpu) }">{{ stats.cpu.toFixed(1) }}<span class="mp-unit">%</span></span>
        </div>
        <TrendChart :data="cpuHistory" :color="loadColor(stats.cpu)" :max="100" area :vw="260" :vh="30" style="height:26px" />
      </div>

      <!-- 两栏 -->
      <div class="mp-cols">
        <!-- 左：内存 + 网络 -->
        <div class="mp-col">
          <div class="mp-row">
            <span class="mp-label">内存</span>
            <div class="mp-spring"></div>
            <span class="mp-mid mono" :style="{ color: loadColor(memPct) }">{{ memPct.toFixed(1) }}<span class="mp-unit-sm">%</span></span>
          </div>
          <div class="mp-bar"><div class="mp-bar-fill" :style="{ width: clamp(memPct) + '%', background: barGradient(memPct) }"></div></div>
          <div class="mp-detail mono">{{ fmtBytes(stats.mem_used) }} / {{ fmtBytes(stats.mem_total) }}</div>

          <div class="mp-net">
            <span class="mp-label">网络</span>
            <div class="mp-net-vals mono">
              <span class="mp-net-down"><i class="ph-bold ph-arrow-down"></i>{{ fmtRate(stats.net_rx) }}</span>
              <span class="mp-net-up"><i class="ph-bold ph-arrow-up"></i>{{ fmtRate(stats.net_tx) }}</span>
            </div>
            <TrendChart :data="netRxHistory" :data2="netTxHistory" color="var(--blue)" color2="var(--green)" :vw="120" :vh="20" style="height:18px; margin-top:4px" />
          </div>
        </div>

        <!-- 右：磁盘 -->
        <div class="mp-col">
          <div class="mp-disk-head">
            <span class="mp-label">磁盘</span>
            <span v-if="sortedDisks.length > 3" class="mp-disk-count mono">{{ sortedDisks.length }}</span>
          </div>
          <div class="mp-disk-list">
            <div v-if="sortedDisks.length === 0" class="mp-detail mono">暂无数据</div>
            <div v-for="d in sortedDisks" :key="d.mount" class="mp-disk">
              <div class="mp-row">
                <span class="mp-disk-mount mono" :title="d.mount">{{ d.mount }}</span>
                <div class="mp-spring"></div>
                <span class="mp-disk-pct mono" :style="{ color: loadColor(diskPct(d)) }">{{ diskPct(d).toFixed(1) }}%</span>
              </div>
              <div class="mp-bar mp-bar-thin"><div class="mp-bar-fill" :style="{ width: clamp(diskPct(d)) + '%', background: loadColor(diskPct(d)) }"></div></div>
              <div class="mp-detail mono mp-disk-detail">{{ fmtBytes(d.used) }} / {{ fmtBytes(d.total) }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- 底部 -->
      <div class="mp-foot">
        <span class="mp-foot-refresh"><i class="ph ph-clock-clockwise"></i>每 1 秒刷新 · 60s 历史</span>
        <div class="mp-spring"></div>
        <span v-if="lastUpdate" class="mono">{{ lastUpdate }}</span>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted, h } from 'vue'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import { StartMonitor, StopMonitor } from '../../wailsjs/go/main/App'
import { useTabsStore } from '../stores/tabs'

interface DiskPartition { mount: string; used: number; total: number }
interface MonitorStats {
  cpu: number; mem_used: number; mem_total: number
  net_rx: number; net_tx: number; cpu_temp: number
  disks: DiskPartition[]; error?: string
}

const props = defineProps<{ connId: string }>()
defineEmits<{ close: [] }>()

const tabsStore = useTabsStore()
const hostLabel = computed(() => tabsStore.tabs.find(t => t.connId === props.connId)?.label ?? '')

// ── 内联走势图（面积 + 折线，颜色可配，可选第二条线）──
const TrendChart = {
  name: 'TrendChart',
  props: {
    data:   { type: Array as () => number[], required: true },
    data2:  { type: Array as () => number[], default: null },
    color:  { type: String, default: 'var(--green)' },
    color2: { type: String, default: 'var(--blue)' },
    area:   { type: Boolean, default: false },
    max:    { type: Number, default: 0 },   // 0 = 自动
    vw:     { type: Number, default: 260 },
    vh:     { type: Number, default: 30 },
  },
  setup(p: any) {
    const gid = 'g' + Math.random().toString(36).slice(2, 8)
    const pts = (arr: number[], maxVal: number) => {
      if (!arr || arr.length < 2) return ''
      return arr.map((v, i) => {
        const x = (i / (arr.length - 1)) * p.vw
        const y = p.vh - (maxVal > 0 ? (v / maxVal) * (p.vh - 2) : 0) - 1
        return `${x.toFixed(1)},${y.toFixed(1)}`
      }).join(' ')
    }
    return () => {
      const all = p.data2 ? [...p.data, ...p.data2] : p.data
      const maxVal = p.max > 0 ? p.max : Math.max(...all, 1)
      const line1 = pts(p.data, maxVal)
      const line2 = p.data2 ? pts(p.data2, maxVal) : ''
      const children: any[] = []
      if (p.area && line1) {
        children.push(h('defs', {}, [
          h('linearGradient', { id: gid, x1: '0', y1: '0', x2: '0', y2: '1' }, [
            h('stop', { offset: '0', 'stop-color': p.color, 'stop-opacity': '0.26' }),
            h('stop', { offset: '1', 'stop-color': p.color, 'stop-opacity': '0' }),
          ]),
        ]))
        children.push(h('polygon', { points: `0,${p.vh} ${line1} ${p.vw},${p.vh}`, fill: `url(#${gid})` }))
      }
      if (line1) children.push(h('polyline', { points: line1, fill: 'none', stroke: p.color, 'stroke-width': 1.5, 'stroke-linejoin': 'round' }))
      if (line2) children.push(h('polyline', { points: line2, fill: 'none', stroke: p.color2, 'stroke-width': 1.3, 'stroke-linejoin': 'round' }))
      return h('svg', { viewBox: `0 0 ${p.vw} ${p.vh}`, preserveAspectRatio: 'none', style: 'width:100%; display:block;' }, children)
    }
  },
}

// ── 数据 ──
const HISTORY_LEN = 60
const EMPTY: MonitorStats = { cpu: 0, mem_used: 0, mem_total: 0, net_rx: 0, net_tx: 0, cpu_temp: -1, disks: [] }
const stats = ref<MonitorStats>({ ...EMPTY })
const lastUpdate = ref('')
const cpuHistory   = ref<number[]>([])
const memHistory   = ref<number[]>([])
const netRxHistory = ref<number[]>([])
const netTxHistory = ref<number[]>([])
function push(buf: number[], v: number) { buf.push(v); if (buf.length > HISTORY_LEN) buf.shift() }

const memPct = computed(() => stats.value.mem_total ? (stats.value.mem_used / stats.value.mem_total) * 100 : 0)
const sortedDisks = computed(() =>
  stats.value.disks?.length ? [...stats.value.disks].sort((a, b) => b.total - a.total) : []
)
function diskPct(d: DiskPartition) { return d.total > 0 ? (d.used / d.total) * 100 : 0 }

// ── 语义化上色（<50 绿 / <80 琥珀 / ≥80 红）──
function loadColor(v: number) { return v < 50 ? 'var(--green)' : v < 80 ? 'var(--amber)' : 'var(--red)' }
function barGradient(v: number) {
  if (v < 50) return 'linear-gradient(90deg,#46cf8b,#6ad6a0)'
  if (v < 80) return 'linear-gradient(90deg,#ecb44a,#f0c56b)'
  return 'linear-gradient(90deg,#ef6d6d,#f0955f)'
}
function clamp(v: number) { return Math.min(100, Math.max(0, v)) }

function fmtBytes(b: number): string {
  if (b >= 1e9) return (b / 1e9).toFixed(2) + ' GB'
  if (b >= 1e6) return (b / 1e6).toFixed(1) + ' MB'
  if (b >= 1e3) return (b / 1e3).toFixed(0) + ' KB'
  return b + ' B'
}
function fmtRate(bps: number): string {
  // 网络显示为 KB/s 数值（单位在模板里另标）
  if (bps >= 1e6) return (bps / 1e6).toFixed(1) + ' M'
  return Math.round(bps / 1e3) + ' K'
}

watch(() => props.connId, async (newId, oldId) => {
  if (oldId) { EventsOff('monitor:stats:' + oldId); StopMonitor(oldId).catch(() => {}) }
  stats.value = { ...EMPTY }
  cpuHistory.value = []; memHistory.value = []; netRxHistory.value = []; netTxHistory.value = []
  lastUpdate.value = ''
  EventsOn('monitor:stats:' + newId, (data: MonitorStats) => {
    stats.value = data
    lastUpdate.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
    if (!data.error) {
      push(cpuHistory.value, data.cpu)
      push(memHistory.value, data.mem_total > 0 ? (data.mem_used / data.mem_total) * 100 : 0)
      push(netRxHistory.value, data.net_rx)
      push(netTxHistory.value, data.net_tx)
    }
  })
  try { await StartMonitor(newId) }
  catch (e) { stats.value = { ...stats.value, error: '启动监控失败：' + String(e) } }
}, { immediate: true })

onUnmounted(() => {
  EventsOff('monitor:stats:' + props.connId)
  StopMonitor(props.connId).catch(() => {})
})
</script>

<style scoped>
.mp {
  background: var(--panel-2);   /* 与上方会话列表(--panel-2)一致，消除右列色差 */
  border-top: 1px solid var(--line);
  padding: 12px 14px;
  font-family: 'IBM Plex Sans', 'Segoe UI', system-ui, sans-serif;
  -webkit-font-smoothing: antialiased;
  color: var(--tp);
}
.mono { font-family: 'IBM Plex Mono', 'Roboto Mono', monospace; }
.mp-spring { flex: 1; }
.mp-row { display: flex; align-items: baseline; }

/* 头 */
.mp-head { display: flex; align-items: center; gap: 7px; margin-bottom: 12px; }
.mp-head-icon { font-size: 15px; color: var(--accent); }
.mp-head-title { font-size: 11px; font-weight: 600; letter-spacing: 1px; color: var(--ts); text-transform: uppercase; }
.mp-breath { width: 6px; height: 6px; border-radius: 50%; background: var(--green); animation: mp-pulse 2s ease-in-out infinite; }
.mp-host { font-size: 10px; color: var(--tm); }
@keyframes mp-pulse { 0%,100% { opacity: 1; box-shadow: 0 0 5px var(--green); } 50% { opacity: 0.35; box-shadow: 0 0 0 var(--green); } }

.mp-error { font-size: 12px; color: var(--red); background: rgba(239,109,109,0.1); border: 1px solid rgba(239,109,109,0.3); border-radius: 6px; padding: 10px 12px; }

.mp-label { font-size: 11px; color: var(--ts); font-weight: 600; letter-spacing: 0.5px; }

/* CPU */
.mp-cpu { margin-bottom: 11px; }
.mp-cpu .mp-row { gap: 8px; margin-bottom: 5px; }
.mp-temp { font-size: 10px; color: var(--amber); background: rgba(236,180,74,0.12); padding: 1px 6px; border-radius: 4px; }
.mp-temp.hot { color: var(--red); background: rgba(239,109,109,0.14); }
.mp-big { font-size: 15px; font-weight: 600; }
.mp-unit { font-size: 11px; color: var(--tm); }
.mp-mid { font-size: 13px; font-weight: 600; }
.mp-unit-sm { font-size: 9px; color: var(--tm); }

/* 两栏 */
.mp-cols { display: flex; gap: 14px; }
.mp-col { flex: 1; min-width: 0; }

/* 进度条 */
.mp-bar { height: 5px; border-radius: 3px; background: var(--hover); overflow: hidden; }
.mp-bar-thin { height: 4px; border-radius: 2px; }
.mp-bar-fill { height: 100%; border-radius: 3px; transition: width 0.6s ease; }
.mp-detail { font-size: 10px; color: var(--tm); margin-top: 4px; }

/* 网络 */
.mp-net { margin-top: 12px; }
.mp-net .mp-label { display: block; margin-bottom: 5px; }
.mp-net-vals { display: flex; gap: 10px; font-size: 10.5px; }
.mp-net-down { color: var(--blue); display: flex; align-items: center; gap: 2px; }
.mp-net-up { color: var(--green); display: flex; align-items: center; gap: 2px; }
.mp-net-vals i { font-size: 10px; }

/* 磁盘 */
.mp-disk-head { display: flex; align-items: baseline; gap: 6px; margin-bottom: 7px; }
.mp-disk-count {
  font-size: 9px; color: var(--tm);
  background: var(--hover); border-radius: 8px; padding: 0 5px;
}
.mp-disk-list {
  max-height: 104px;          /* 矮一点：约 3 行，多出的滚动 */
  overflow-y: auto;
  scrollbar-width: none;      /* 隐藏滚动条（滚轮仍可滚动，数量徽标提示有更多） */
}
.mp-disk-list::-webkit-scrollbar { width: 0; height: 0; display: none; }
.mp-disk { margin-bottom: 8px; }
.mp-disk:last-child { margin-bottom: 0; }
.mp-disk .mp-row { margin-bottom: 3px; }
.mp-disk-mount { font-size: 11px; color: var(--tp); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 90px; }
.mp-disk-pct { font-size: 11px; font-weight: 600; }
.mp-disk-detail { font-size: 9px; margin-top: 2px; }

/* 底部 */
.mp-foot { display: flex; align-items: center; margin-top: 6px; padding-top: 9px; border-top: 1px solid var(--line-soft); font-size: 10px; color: var(--tm); }
.mp-foot-refresh { display: flex; align-items: center; gap: 4px; }
.mp-foot-refresh i { font-size: 11px; }
</style>
