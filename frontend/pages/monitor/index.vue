<template>
  <div>
    <div class="page-header">
      <h1>监控Dashboard</h1>
      <div style="display: flex; gap: 12px; align-items: center;">
        <select v-model="timeRange" @change="fetchData" style="padding: 6px 12px; border: 1px solid #d1d5db; border-radius: 6px;">
          <option value="5">最近5分钟</option>
          <option value="15">最近15分钟</option>
          <option value="60">最近1小时</option>
          <option value="1440">最近24小时</option>
        </select>
        <button class="btn btn-outline btn-sm" @click="fetchData">刷新</button>
      </div>
    </div>

    <div class="grid-4">
      <div class="stat-card">
        <div class="stat-value">{{ stats.delivery_stats?.delivered || 0 }}</div>
        <div class="stat-label">已投递</div>
      </div>
      <div class="stat-card">
        <div class="stat-value" style="color: #f59e0b;">{{ stats.delivery_stats?.pending || 0 }}</div>
        <div class="stat-label">待投递</div>
      </div>
      <div class="stat-card">
        <div class="stat-value" style="color: #ef4444;">{{ stats.dlq_depth || 0 }}</div>
        <div class="stat-label">死信队列深度</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{{ formatLatency(stats.delivery_latency?.avg_latency_ms) }}</div>
        <div class="stat-label">平均延迟</div>
      </div>
    </div>

    <div class="grid-2" style="margin-top: 24px;">
      <div class="card">
        <h3 style="margin-bottom: 16px;">发布QPS实时曲线</h3>
        <div class="chart-container">
          <canvas ref="qpsChart"></canvas>
        </div>
      </div>
      <div class="card">
        <h3 style="margin-bottom: 16px;">消费延迟分布</h3>
        <div class="chart-container">
          <canvas ref="latencyChart"></canvas>
        </div>
      </div>
    </div>

    <div class="card" style="margin-top: 16px;">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px;">
        <h3>事件发布热力图</h3>
        <div style="display: flex; align-items: center; gap: 16px; font-size: 12px; color: #6b7280;">
          <span style="display: flex; align-items: center; gap: 4px;">
            <span style="display: inline-block; width: 12px; height: 12px; background: rgba(147, 197, 253, 0.3); border-radius: 2px;"></span> 0
          </span>
          <span style="display: flex; align-items: center; gap: 4px;">
            <span style="display: inline-block; width: 12px; height: 12px; background: rgb(140, 84, 142); border-radius: 2px;"></span> 中等
          </span>
          <span style="display: flex; align-items: center; gap: 4px;">
            <span style="display: inline-block; width: 12px; height: 12px; background: rgb(220, 38, 38); border-radius: 2px;"></span> 最大
          </span>
          <span style="display: flex; align-items: center; gap: 4px;">
            <span style="display: inline-block; width: 12px; height: 12px; border: 2px solid #eab308; border-radius: 2px;"></span> 超QPS上限({{ maxPublishQPS }}/秒)
          </span>
        </div>
      </div>
      <div v-if="heatmapData.length === 0 && heatmapLoaded" style="text-align: center; color: #9ca3af; padding: 40px;">暂无热力图数据</div>
      <EventHeatmap
        v-if="heatmapData.length > 0"
        :heatmap-data="heatmapData"
        :event-types="heatmapEventTypes"
        :max-qps-limit="maxPublishQPS"
        :time-range="parseInt(timeRange)"
        @cell-click="onHeatmapCellClick"
      />
    </div>

    <div class="grid-2" style="margin-top: 16px;">
      <div class="card">
        <h3 style="margin-bottom: 16px;">订阅积压量</h3>
        <table>
          <thead>
            <tr>
              <th>订阅ID</th>
              <th>积压数量</th>
              <th>状态</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(count, subId) in stats.backlog_counts" :key="subId">
              <td style="font-family: monospace; font-size: 12px;">{{ (subId as string).substring(0, 8) }}...</td>
              <td>
                <span :style="{ color: count > 500 ? '#ef4444' : count > 100 ? '#f59e0b' : '#22c55e', fontWeight: 600 }">
                  {{ count }}
                </span>
              </td>
              <td><span :class="['badge', count > 500 ? 'badge-failed' : count > 100 ? 'badge-pending' : 'badge-active']">
                {{ count > 500 ? '积压严重' : count > 100 ? '轻微积压' : '正常' }}
              </span></td>
            </tr>
            <tr v-if="!stats.backlog_counts || Object.keys(stats.backlog_counts).length === 0">
              <td colspan="3" style="text-align: center; color: #9ca3af;">暂无积压数据</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="card">
        <h3 style="margin-bottom: 16px;">消费者在线状态</h3>
        <table>
          <thead>
            <tr>
              <th>订阅ID</th>
              <th>状态</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(online, subId) in stats.consumer_statuses" :key="subId">
              <td style="font-family: monospace; font-size: 12px;">{{ (subId as string).substring(0, 8) }}...</td>
              <td>
                <span :class="['badge', online ? 'badge-active' : 'badge-disabled']">
                  {{ online ? '在线' : '离线' }}
                </span>
              </td>
            </tr>
            <tr v-if="!stats.consumer_statuses || Object.keys(stats.consumer_statuses).length === 0">
              <td colspan="2" style="text-align: center; color: #9ca3af;">暂无消费者数据</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="card" style="margin-top: 16px;">
      <h3 style="margin-bottom: 16px;">背压告警</h3>
      <table>
        <thead>
          <tr>
            <th>告警类型</th>
            <th>订阅ID</th>
            <th>消息</th>
            <th>时间</th>
            <th>状态</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="alert in alerts" :key="alert.id">
            <td><span class="badge badge-pending">{{ alert.alert_type }}</span></td>
            <td style="font-family: monospace; font-size: 12px;">{{ alert.subscription_id?.substring(0, 8) }}...</td>
            <td>{{ alert.message }}</td>
            <td>{{ formatTime(alert.created_at) }}</td>
            <td><span :class="['badge', alert.is_resolved ? 'badge-active' : 'badge-failed']">
              {{ alert.is_resolved ? '已解决' : '未解决' }}
            </span></td>
          </tr>
          <tr v-if="alerts.length === 0">
            <td colspan="5" style="text-align: center; color: #9ca3af;">暂无告警</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
const api = useApiStore()
const router = useRouter()
const stats = ref<any>({})
const alerts = ref<any[]>([])
const timeRange = ref('60')
const qpsChart = ref<HTMLCanvasElement | null>(null)
const latencyChart = ref<HTMLCanvasElement | null>(null)

const heatmapData = ref<any[]>([])
const heatmapEventTypes = ref<string[]>([])
const maxPublishQPS = ref(0)
const heatmapLoaded = ref(false)

const fetchData = async () => {
  if (!api.tenantId) return
  try {
    stats.value = await api.get('/monitor/dashboard')
    alerts.value = await api.get('/monitor/alerts')
    renderCharts()
  } catch (e) {}
  fetchHeatmap()
}

const fetchHeatmap = async () => {
  if (!api.tenantId) return
  try {
    const data = await api.get(`/monitor/heatmap?minutes=${timeRange.value}`)
    heatmapData.value = data.heatmap_data || []
    heatmapEventTypes.value = data.event_types || []
    maxPublishQPS.value = data.max_publish_qps || 0
    heatmapLoaded.value = true
  } catch (e) {
    heatmapLoaded.value = true
  }
}

const onHeatmapCellClick = (eventType: string, startTime: string, endTime: string) => {
  router.push({
    path: '/traces',
    query: {
      event_type: eventType,
      start_time: startTime,
      end_time: endTime,
    },
  })
}

const formatLatency = (ms: number | undefined) => {
  if (!ms) return '0ms'
  return ms < 1 ? `${(ms * 1000).toFixed(0)}μs` : `${ms.toFixed(1)}ms`
}

const formatTime = (t: string) => {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN')
}

const renderCharts = () => {
  renderQPSChart()
  renderLatencyChart()
}

const renderQPSChart = () => {
  if (!qpsChart.value) return
  const ctx = qpsChart.value.getContext('2d')
  if (!ctx) return

  const data = stats.value.publish_qps_history || []
  if (data.length === 0) return

  const width = qpsChart.value.parentElement?.clientWidth || 400
  const height = 260
  qpsChart.value.width = width
  qpsChart.value.height = height

  const maxCount = Math.max(...data.map((d: any) => d.count), 1)
  const stepX = width / (data.length - 1 || 1)
  const padding = 40

  ctx.clearRect(0, 0, width, height)

  ctx.beginPath()
  ctx.strokeStyle = '#3b82f6'
  ctx.lineWidth = 2

  data.forEach((d: any, i: number) => {
    const x = padding + i * stepX
    const y = height - padding - (d.count / maxCount) * (height - 2 * padding)
    if (i === 0) ctx.moveTo(x, y)
    else ctx.lineTo(x, y)
  })
  ctx.stroke()

  ctx.fillStyle = '#3b82f620'
  ctx.beginPath()
  data.forEach((d: any, i: number) => {
    const x = padding + i * stepX
    const y = height - padding - (d.count / maxCount) * (height - 2 * padding)
    if (i === 0) ctx.moveTo(x, y)
    else ctx.lineTo(x, y)
  })
  ctx.lineTo(padding + (data.length - 1) * stepX, height - padding)
  ctx.lineTo(padding, height - padding)
  ctx.closePath()
  ctx.fill()

  ctx.fillStyle = '#6b7280'
  ctx.font = '11px sans-serif'
  ctx.fillText('QPS', 4, padding)
  ctx.fillText('时间', width - 40, height - 4)
}

const renderLatencyChart = () => {
  if (!latencyChart.value) return
  const ctx = latencyChart.value.getContext('2d')
  if (!ctx) return

  const latency = stats.value.delivery_latency || {}
  const labels = ['P50', 'P95', 'P99', 'AVG']
  const values = [latency.p50_ms || 0, latency.p95_ms || 0, latency.p99_ms || 0, latency.avg_latency_ms || 0]

  const width = latencyChart.value.parentElement?.clientWidth || 400
  const height = 260
  latencyChart.value.width = width
  latencyChart.value.height = height

  ctx.clearRect(0, 0, width, height)

  const maxVal = Math.max(...values, 1)
  const barWidth = 50
  const gap = (width - labels.length * barWidth) / (labels.length + 1)
  const padding = 40

  const colors = ['#3b82f6', '#f59e0b', '#ef4444', '#22c55e']

  labels.forEach((label, i) => {
    const x = gap + i * (barWidth + gap)
    const barHeight = (values[i] / maxVal) * (height - 2 * padding)

    ctx.fillStyle = colors[i]
    ctx.fillRect(x, height - padding - barHeight, barWidth, barHeight)

    ctx.fillStyle = '#1f2937'
    ctx.font = '12px sans-serif'
    ctx.textAlign = 'center'
    ctx.fillText(label, x + barWidth / 2, height - padding + 16)
    ctx.fillText(`${values[i].toFixed(1)}ms`, x + barWidth / 2, height - padding - barHeight - 8)
  })
}

let refreshTimer: any = null

onMounted(() => {
  fetchData()
  refreshTimer = setInterval(fetchData, 10000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>
