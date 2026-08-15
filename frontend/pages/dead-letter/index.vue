<template>
  <div>
    <div class="page-header">
      <h1>死信队列</h1>
      <div style="display: flex; gap: 12px;">
        <button class="btn btn-outline" @click="fetchEntries">刷新</button>
        <button class="btn btn-success" :disabled="selectedIds.length === 0" @click="batchRetry">
          批量重发 ({{ selectedIds.length }})
        </button>
      </div>
    </div>

    <div class="card">
      <table>
        <thead>
          <tr>
            <th style="width: 40px;">
              <input type="checkbox" @change="toggleAll" :checked="allSelected" />
            </th>
            <th>事件ID</th>
            <th>订阅ID</th>
            <th>失败原因</th>
            <th>重试次数</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="entry in entries" :key="entry.id">
            <td>
              <input type="checkbox" :value="entry.id" v-model="selectedIds" />
            </td>
            <td style="font-family: monospace; font-size: 12px;">{{ entry.event_id?.substring(0, 8) }}...</td>
            <td style="font-family: monospace; font-size: 12px;">{{ entry.subscription_id?.substring(0, 8) }}...</td>
            <td style="max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: #ef4444;">
              {{ entry.failure_reason }}
            </td>
            <td>{{ entry.retry_count }}</td>
            <td style="font-size: 12px; color: #6b7280;">{{ formatTime(entry.created_at) }}</td>
            <td>
              <button class="btn btn-outline btn-sm" @click="retrySingle(entry.id)">重发</button>
              <button class="btn btn-outline btn-sm" style="margin-left: 8px;" @click="editAndRetry(entry)">编辑重发</button>
              <button class="btn btn-outline btn-sm" style="margin-left: 8px;" @click="viewPayload(entry)">查看</button>
            </td>
          </tr>
          <tr v-if="entries.length === 0">
            <td colspan="7" style="text-align: center; color: #9ca3af; padding: 40px;">死信队列为空</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div style="display: flex; justify-content: center; gap: 8px; margin-top: 16px;">
      <button class="btn btn-outline btn-sm" :disabled="offset === 0" @click="prevPage">上一页</button>
      <span style="padding: 4px 12px; color: #6b7280;">第 {{ page }} 页</span>
      <button class="btn btn-outline btn-sm" @click="nextPage">下一页</button>
    </div>

    <div v-if="showEditModal" class="modal-overlay" @click.self="showEditModal = false">
      <div class="modal" style="max-width: 700px;">
        <h2>编辑事件内容后重发</h2>
        <div class="form-group">
          <label>事件Payload</label>
          <textarea v-model="editPayload" style="min-height: 300px; font-family: 'Monaco', monospace; font-size: 13px;"></textarea>
        </div>
        <div class="modal-actions">
          <button class="btn btn-outline" @click="showEditModal = false">取消</button>
          <button class="btn btn-primary" @click="submitEditAndRetry">编辑并重发</button>
        </div>
      </div>
    </div>

    <div v-if="showPayloadModal" class="modal-overlay" @click.self="showPayloadModal = false">
      <div class="modal" style="max-width: 700px;">
        <h2>事件详情</h2>
        <div style="background: #f9fafb; padding: 16px; border-radius: 8px;">
          <h4 style="margin-bottom: 8px; color: #6b7280;">失败原因</h4>
          <p style="color: #ef4444; margin-bottom: 16px;">{{ selectedEntry?.failure_reason }}</p>
          <h4 style="margin-bottom: 8px; color: #6b7280;">事件内容</h4>
          <pre style="font-size: 12px; overflow-x: auto; white-space: pre-wrap;">{{ formatJSON(selectedEntry?.original_payload) }}</pre>
          <div class="grid-2" style="margin-top: 16px;">
            <div><strong>重试次数:</strong> {{ selectedEntry?.retry_count }}</div>
            <div><strong>创建时间:</strong> {{ formatTime(selectedEntry?.created_at) }}</div>
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn btn-outline" @click="showPayloadModal = false">关闭</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
const api = useApiStore()
const entries = ref<any[]>([])
const selectedIds = ref<string[]>([])
const offset = ref(0)
const limit = 50
const page = computed(() => Math.floor(offset.value / limit) + 1)

const showEditModal = ref(false)
const editPayload = ref('')
const editingId = ref('')

const showPayloadModal = ref(false)
const selectedEntry = ref<any>(null)

const fetchEntries = async () => {
  if (!api.tenantId) return
  try {
    entries.value = await api.get(`/dead-letter?limit=${limit}&offset=${offset.value}`)
  } catch (e) {}
}

const retrySingle = async (id: string) => {
  try {
    await api.post(`/dead-letter/${id}/retry`, {})
    fetchEntries()
  } catch (e) {}
}

const batchRetry = async () => {
  try {
    await api.post('/dead-letter/batch-retry', { ids: selectedIds.value })
    selectedIds.value = []
    fetchEntries()
  } catch (e) {}
}

const editAndRetry = (entry: any) => {
  editingId.value = entry.id
  editPayload.value = typeof entry.original_payload === 'string'
    ? JSON.stringify(JSON.parse(entry.original_payload), null, 2)
    : JSON.stringify(entry.original_payload, null, 2)
  showEditModal.value = true
}

const submitEditAndRetry = async () => {
  try {
    await api.put(`/dead-letter/${editingId.value}/edit-retry`, { payload: editPayload.value })
    showEditModal.value = false
    fetchEntries()
  } catch (e) {}
}

const viewPayload = (entry: any) => {
  selectedEntry.value = entry
  showPayloadModal.value = true
}

const toggleAll = (e: Event) => {
  const checked = (e.target as HTMLInputElement).checked
  selectedIds.value = checked ? entries.value.map((e: any) => e.id) : []
}

const allSelected = computed(() => entries.value.length > 0 && selectedIds.value.length === entries.value.length)

const prevPage = () => { offset.value = Math.max(0, offset.value - limit) }
const nextPage = () => { offset.value += limit }

const formatTime = (t: string | undefined) => {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN')
}

const formatJSON = (s: string | undefined) => {
  if (!s) return ''
  try {
    return JSON.stringify(JSON.parse(s), null, 2)
  } catch { return s }
}

onMounted(fetchEntries)
</script>
