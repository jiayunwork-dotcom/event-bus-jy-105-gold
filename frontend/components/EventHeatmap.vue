<template>
  <div class="heatmap-wrapper" style="position: relative;">
    <canvas
      ref="canvasRef"
      @mousemove="onMouseMove"
      @mouseleave="onMouseLeave"
      @click="onMouseClick"
      style="cursor: crosshair; display: block;"
    />
    <div
      v-if="tooltip.visible"
      class="heatmap-tooltip"
      :style="{ left: tooltip.x + 'px', top: tooltip.y + 'px' }"
    >
      <div class="tooltip-row"><strong>{{ tooltip.eventType }}</strong></div>
      <div class="tooltip-row">{{ tooltip.minute }}</div>
      <div class="tooltip-row">发布数量: <strong>{{ tooltip.count }}</strong></div>
      <div v-if="tooltip.exceedQPS" class="tooltip-row" style="color: #f59e0b;">⚠ 超过{{ props.maxQpsLimit }}/秒的QPS上限({{ props.maxQpsLimit * 60 }}条/分钟)</div>
    </div>
  </div>
</template>

<script setup lang="ts">
const props = defineProps<{
  heatmapData: any[]
  eventTypes: string[]
  maxQpsLimit: number
  timeRange: number
}>()

const emit = defineEmits<{
  (e: 'cellClick', eventType: string, startTime: string, endTime: string): void
}>()

const canvasRef = ref<HTMLCanvasElement | null>(null)
const tooltip = ref({
  visible: false,
  x: 0,
  y: 0,
  eventType: '',
  minute: '',
  count: 0,
  exceedQPS: false,
})

const cellMap = ref<{ x: number; y: number; w: number; h: number; eventType: string; minute: string; count: number; exceedQPS: boolean }[]>([])

const GAP = 1
const LABEL_WIDTH = 120
const TIME_LABEL_HEIGHT = 36
const CELL_MIN_WIDTH = 6
const CELL_HEIGHT = 28

const getColor = (value: number, max: number): string => {
  if (max === 0) return 'rgba(147, 197, 253, 0.3)'
  const ratio = Math.min(value / max, 1)
  const r = Math.round(59 + (220 - 59) * ratio)
  const g = Math.round(130 + (38 - 130) * ratio)
  const b = Math.round(246 + (38 - 246) * ratio)
  return `rgb(${r}, ${g}, ${b})`
}

const buildCellMap = (
  dataMap: Record<string, Record<string, number>>,
  timeSlots: string[],
  cellWidth: number,
) => {
  const cells: typeof cellMap.value = []
  let cy = TIME_LABEL_HEIGHT
  for (const eventType of props.eventTypes) {
    let cx = LABEL_WIDTH
    for (const minute of timeSlots) {
      const count = dataMap[eventType]?.[minute] || 0
      const exceedQPS = props.maxQpsLimit > 0 && count > props.maxQpsLimit * 60
      cells.push({ x: cx, y: cy, w: cellWidth, h: CELL_HEIGHT, eventType, minute, count, exceedQPS })
      cx += cellWidth + GAP
    }
    cy += CELL_HEIGHT + GAP
  }
  cellMap.value = cells
}

const renderHeatmap = () => {
  if (!canvasRef.value) return
  const canvas = canvasRef.value
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const dataMap: Record<string, Record<string, number>> = {}
  let maxCount = 0
  for (const item of props.heatmapData) {
    const et = item.event_type as string
    const min = item.minute as string
    const cnt = item.count as number
    if (!dataMap[et]) dataMap[et] = {}
    dataMap[et][min] = cnt
    if (cnt > maxCount) maxCount = cnt
  }

  const timeSet = new Set<string>()
  for (const item of props.heatmapData) {
    timeSet.add(item.minute as string)
  }
  const timeSlots = Array.from(timeSet).sort()

  const containerWidth = canvas.parentElement?.clientWidth || 800
  const totalWidth = Math.max(containerWidth, LABEL_WIDTH + timeSlots.length * (CELL_MIN_WIDTH + GAP))
  const cellWidth = Math.max(CELL_MIN_WIDTH, (totalWidth - LABEL_WIDTH - timeSlots.length * GAP) / Math.max(timeSlots.length, 1))
  const totalHeight = TIME_LABEL_HEIGHT + props.eventTypes.length * (CELL_HEIGHT + GAP)

  canvas.width = totalWidth
  canvas.height = totalHeight

  ctx.clearRect(0, 0, totalWidth, totalHeight)

  ctx.fillStyle = '#6b7280'
  ctx.font = '11px sans-serif'
  ctx.textBaseline = 'middle'

  let labelY = TIME_LABEL_HEIGHT
  for (const eventType of props.eventTypes) {
    ctx.fillStyle = '#374151'
    ctx.textAlign = 'right'
    ctx.fillText(eventType.length > 14 ? eventType.substring(0, 12) + '...' : eventType, LABEL_WIDTH - 8, labelY + CELL_HEIGHT / 2)
    labelY += CELL_HEIGHT + GAP
  }

  const timeLabelStep = Math.max(1, Math.floor(timeSlots.length / Math.floor((totalWidth - LABEL_WIDTH) / 60)))
  ctx.textAlign = 'center'
  for (let i = 0; i < timeSlots.length; i++) {
    if (i % timeLabelStep === 0) {
      const x = LABEL_WIDTH + i * (cellWidth + GAP) + cellWidth / 2
      ctx.fillStyle = '#6b7280'
      const t = timeSlots[i]
      const d = new Date(t)
      const short = `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
      ctx.fillText(short, x, TIME_LABEL_HEIGHT - 8)
    }
  }

  buildCellMap(dataMap, timeSlots, cellWidth)

  for (const cell of cellMap.value) {
    ctx.fillStyle = getColor(cell.count, maxCount)
    ctx.fillRect(cell.x, cell.y, cellWidth, CELL_HEIGHT)

    if (cell.exceedQPS) {
      ctx.strokeStyle = '#eab308'
      ctx.lineWidth = 2
      ctx.strokeRect(cell.x + 1, cell.y + 1, cellWidth - 2, CELL_HEIGHT - 2)
    }
  }
}

const findCell = (mx: number, my: number) => {
  for (const cell of cellMap.value) {
    if (mx >= cell.x && mx <= cell.x + cell.w && my >= cell.y && my <= cell.y + cell.h) {
      return cell
    }
  }
  return null
}

const onMouseMove = (e: MouseEvent) => {
  if (!canvasRef.value) return
  const rect = canvasRef.value.getBoundingClientRect()
  const scaleX = canvasRef.value.width / rect.width
  const scaleY = canvasRef.value.height / rect.height
  const mx = (e.clientX - rect.left) * scaleX
  const my = (e.clientY - rect.top) * scaleY
  const cell = findCell(mx, my)
  if (cell) {
    const minuteDate = new Date(cell.minute)
    const nextMinute = new Date(minuteDate.getTime() + 60000)
    tooltip.value = {
      visible: true,
      x: e.offsetX + 12,
      y: e.offsetY - 8,
      eventType: cell.eventType,
      minute: `${minuteDate.toLocaleString('zh-CN', { hour: '2-digit', minute: '2-digit' })}`,
      count: cell.count,
      exceedQPS: cell.exceedQPS,
    }
  } else {
    tooltip.value.visible = false
  }
}

const onMouseLeave = () => {
  tooltip.value.visible = false
}

const onMouseClick = (e: MouseEvent) => {
  if (!canvasRef.value) return
  const rect = canvasRef.value.getBoundingClientRect()
  const scaleX = canvasRef.value.width / rect.width
  const scaleY = canvasRef.value.height / rect.height
  const mx = (e.clientX - rect.left) * scaleX
  const my = (e.clientY - rect.top) * scaleY
  const cell = findCell(mx, my)
  if (cell) {
    const minuteDate = new Date(cell.minute)
    const nextMinute = new Date(minuteDate.getTime() + 60000)
    const startTime = minuteDate.toISOString()
    const endTime = nextMinute.toISOString()
    emit('cellClick', cell.eventType, startTime, endTime)
  }
}

watch(() => [props.heatmapData, props.eventTypes, props.maxQpsLimit], () => {
  nextTick(() => renderHeatmap())
}, { deep: true })

onMounted(() => {
  renderHeatmap()
  window.addEventListener('resize', renderHeatmap)
})

onUnmounted(() => {
  window.removeEventListener('resize', renderHeatmap)
})
</script>

<style scoped>
.heatmap-wrapper {
  position: relative;
  overflow-x: auto;
}

.heatmap-tooltip {
  position: absolute;
  background: rgba(17, 24, 39, 0.92);
  color: #f3f4f6;
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 12px;
  line-height: 1.6;
  pointer-events: none;
  z-index: 10;
  white-space: nowrap;
  box-shadow: 0 2px 8px rgba(0,0,0,0.2);
}

.tooltip-row {
  margin: 0;
}
</style>
