<template>
  <div>
    <div class="page-header">
      <h1>事件追踪</h1>
    </div>

    <div class="card">
      <div style="display: flex; gap: 12px; margin-bottom: 24px;">
        <input v-model="eventId" placeholder="输入事件ID" style="flex: 1; padding: 10px 16px; border: 1px solid #d1d5db; border-radius: 8px; font-size: 14px;" />
        <button class="btn btn-primary" @click="searchTrace">追踪</button>
      </div>

      <div v-if="prefilterEventType" style="margin-bottom: 24px; padding: 12px 16px; background: #eff6ff; border: 1px solid #bfdbfe; border-radius: 8px;">
        <div style="display: flex; justify-content: space-between; align-items: center;">
          <div style="font-size: 13px; color: #1e40af;">
            <strong>热力图筛选:</strong> 事件类型=<code style="background: #dbeafe; padding: 2px 6px; border-radius: 3px;">{{ prefilterEventType }}</code>
            <span v-if="prefilterStartTime" style="margin-left: 8px;">
              时间={{ formatFilterTime(prefilterStartTime) }} ~ {{ formatFilterTime(prefilterEndTime!) }}
            </span>
          </div>
          <button class="btn btn-outline btn-sm" @click="clearPrefilter">清除筛选</button>
        </div>
      </div>

      <div v-if="prefilterEvents.length > 0 && prefilterEventType" style="margin-bottom: 24px;">
        <h3 style="margin-bottom: 12px; font-size: 14px; color: #374151;">匹配的事件列表</h3>
        <table>
          <thead>
            <tr>
              <th>事件ID</th>
              <th>事件类型</th>
              <th>创建时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="event in prefilterEvents" :key="event.id">
              <td style="font-family: monospace; font-size: 12px;">{{ event.id.substring(0, 8) }}...</td>
              <td>{{ event.event_type }}</td>
              <td>{{ formatFilterTime(event.created_at) }}</td>
              <td><button class="btn btn-outline btn-sm" @click="eventId = event.id; searchTrace()">追踪</button></td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="traces.length === 0 && searched" style="text-align: center; color: #9ca3af; padding: 60px;">
        未找到该事件的投递链路
      </div>

      <div v-if="traces.length > 0">
        <div v-for="group in groupedTraces" :key="group.subscription_id" style="margin-bottom: 32px;">
          <h3 style="margin-bottom: 16px; font-size: 16px;">
            订阅: <code style="background: #f3f4f6; padding: 2px 8px; border-radius: 4px;">{{ group.subscription_id.substring(0, 8) }}...</code>
          </h3>

          <div style="padding-left: 20px;">
            <div v-for="(trace, idx) in group.traces" :key="trace.id" class="waterfall-item"
                 :class="{ failed: trace.status === 'failed' }"
                 :style="{ marginLeft: (idx * 20) + 'px' }">
              <div class="waterfall-info">
                <div class="node-name">{{ trace.node_name }}</div>
                <div class="node-type">
                  {{ trace.node_type }}
                  <span :class="['badge', `badge-${trace.status === 'success' ? 'active' : 'failed'}`]" style="margin-left: 8px;">
                    {{ trace.status }}
                  </span>
                </div>
                <div v-if="trace.error_message" style="color: #ef4444; font-size: 12px; margin-top: 4px;">
                  {{ trace.error_message }}
                </div>
              </div>
              <div style="display: flex; align-items: center; gap: 16px;">
                <div class="waterfall-duration">{{ trace.duration_ms }}ms</div>
                <div class="waterfall-bar" :style="{ width: Math.max(4, Math.min(trace.duration_ms, 200)) + 'px' }"></div>
              </div>
            </div>
          </div>

          <div style="margin-left: 20px; padding: 12px 16px; background: #f9fafb; border-radius: 8px; margin-top: 8px;">
            <div style="display: flex; gap: 24px; font-size: 13px; color: #6b7280;">
              <span>总耗时: <strong style="color: #1f2937;">{{ getTotalDuration(group.traces) }}ms</strong></span>
              <span>节点数: <strong style="color: #1f2937;">{{ group.traces.length }}</strong></span>
              <span>最终状态:
                <strong :style="{ color: group.traces.some((t: any) => t.status === 'failed') ? '#ef4444' : '#22c55e' }">
                  {{ group.traces.some((t: any) => t.status === 'failed') ? '失败' : '成功' }}
                </strong>
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
const api = useApiStore()
const route = useRoute()
const router = useRouter()
const eventId = ref('')
const traces = ref<any[]>([])
const searched = ref(false)

const prefilterEventType = ref('')
const prefilterStartTime = ref('')
const prefilterEndTime = ref('')
const prefilterEvents = ref<any[]>([])

interface TraceGroup {
  subscription_id: string
  traces: any[]
}

const groupedTraces = computed<TraceGroup[]>(() => {
  const groups: Record<string, any[]> = {}
  for (const trace of traces.value) {
    if (!groups[trace.subscription_id]) {
      groups[trace.subscription_id] = []
    }
    groups[trace.subscription_id].push(trace)
  }
  return Object.entries(groups).map(([subscription_id, traces]) => ({
    subscription_id,
    traces: traces.sort((a: any, b: any) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime())
  }))
})

const searchTrace = async () => {
  if (!eventId.value) return
  searched.value = true
  try {
    traces.value = await api.get(`/monitor/traces/${eventId.value}`)
  } catch (e) {
    traces.value = []
  }
}

const fetchPrefilterEvents = async () => {
  if (!prefilterEventType.value || !prefilterStartTime.value || !prefilterEndTime.value) return
  if (!api.tenantId) return
  try {
    const params = new URLSearchParams({
      event_type: prefilterEventType.value,
      start_time: prefilterStartTime.value,
      end_time: prefilterEndTime.value,
      limit: '100',
    })
    prefilterEvents.value = await api.get(`/monitor/events?${params.toString()}`)
  } catch (e) {
    prefilterEvents.value = []
  }
}

const clearPrefilter = () => {
  prefilterEventType.value = ''
  prefilterStartTime.value = ''
  prefilterEndTime.value = ''
  prefilterEvents.value = []
  router.replace({ path: '/traces' })
}

const formatFilterTime = (t: string) => {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN')
}

const getTotalDuration = (traces: any[]) => {
  return traces.reduce((sum: number, t: any) => sum + (t.duration_ms || 0), 0)
}

onMounted(() => {
  const qEventType = route.query.event_type as string
  const qStartTime = route.query.start_time as string
  const qEndTime = route.query.end_time as string

  if (qEventType) {
    prefilterEventType.value = qEventType
    prefilterStartTime.value = qStartTime || ''
    prefilterEndTime.value = qEndTime || ''
    fetchPrefilterEvents()
  }
})
</script>
