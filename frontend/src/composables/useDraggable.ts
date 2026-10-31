import { ref, computed, onUnmounted } from 'vue'

/**
 * Makes a dialog box draggable within the app window.
 * Usage:
 *   const { boxStyle, startDrag } = useDraggable()
 *   <div :style="boxStyle" style="--wails-draggable:no-drag">
 *     <div class="header" @mousedown.prevent="startDrag" style="cursor:grab">
 */
export function useDraggable() {
  const dx = ref(0)
  const dy = ref(0)
  let ox = 0, oy = 0, bdx = 0, bdy = 0

  function onMove(e: MouseEvent) {
    dx.value = bdx + (e.clientX - ox)
    dy.value = bdy + (e.clientY - oy)
  }

  function onUp() {
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
  }

  function startDrag(e: MouseEvent) {
    ox  = e.clientX
    oy  = e.clientY
    bdx = dx.value
    bdy = dy.value
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
    e.preventDefault()
  }

  onUnmounted(onUp)

  const boxStyle = computed(() => ({
    transform: `translate(${dx.value}px, ${dy.value}px)`,
  }))

  return { boxStyle, startDrag }
}
