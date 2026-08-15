<template>
  <div>
    <div class="page-header">
      <h1>Schema管理</h1>
      <button class="btn btn-primary" @click="showRegisterModal = true">+ 注册Schema</button>
    </div>

    <div class="card">
      <table>
        <thead>
          <tr>
            <th>事件类型</th>
            <th>最新版本</th>
            <th>兼容性</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="schema in schemas" :key="schema.id">
            <td style="font-weight: 600;">{{ schema.event_type }}</td>
            <td>v{{ schema.version }}</td>
            <td>
              <span :class="['badge', schema.is_compatible ? 'badge-active' : 'badge-failed']">
                {{ schema.is_compatible ? '兼容' : '不兼容' }}
              </span>
            </td>
            <td>{{ formatTime(schema.created_at) }}</td>
            <td>
              <button class="btn btn-outline btn-sm" @click="viewVersions(schema.event_type)">版本历史</button>
              <button class="btn btn-outline btn-sm" style="margin-left: 8px;" @click="checkCompat(schema.event_type)">兼容性检查</button>
              <button class="btn btn-primary btn-sm" style="margin-left: 8px;" @click="openMigrationWizard(schema.event_type)">版本迁移</button>
            </td>
          </tr>
          <tr v-if="schemas.length === 0">
            <td colspan="5" style="text-align: center; color: #9ca3af; padding: 40px;">暂无Schema数据</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="showRegisterModal" class="modal-overlay" @click.self="showRegisterModal = false">
      <div class="modal" style="max-width: 800px;">
        <h2>注册新Schema</h2>
        <div class="form-group">
          <label>事件类型</label>
          <input v-model="form.event_type" placeholder="e.g. order.created" />
        </div>
        <div class="form-group">
          <label>Schema定义 (JSON Schema)</label>
          <textarea v-model="form.schema_def" style="min-height: 300px; font-family: 'Monaco', 'Menlo', monospace; font-size: 13px;" placeholder='{
  "type": "object",
  "properties": {
    "order_id": { "type": "string" },
    "amount": { "type": "number" }
  },
  "required": ["order_id", "amount"]
}'></textarea>
        </div>
        <div v-if="compatResult" style="margin-top: 16px; padding: 16px; border-radius: 8px;"
             :style="{ background: compatResult.is_compatible ? '#dcfce7' : '#fef2f2' }">
          <strong>兼容性检查:</strong>
          <span :style="{ color: compatResult.is_compatible ? '#166534' : '#991b1b' }">
            {{ compatResult.is_compatible ? '向前兼容' : '不兼容' }}
          </span>
          <span v-if="compatResult.note" style="margin-left: 12px; color: #6b7280;">{{ compatResult.note }}</span>
        </div>
        <div class="modal-actions">
          <button class="btn btn-outline" @click="checkFormCompat" :disabled="!form.event_type || !form.schema_def">检查兼容性</button>
          <button class="btn btn-outline" @click="showRegisterModal = false">取消</button>
          <button class="btn btn-primary" @click="submitSchema">注册</button>
        </div>
      </div>
    </div>

    <div v-if="showVersionsModal" class="modal-overlay" @click.self="showVersionsModal = false">
      <div class="modal" style="max-width: 900px;">
        <h2>版本历史 - {{ versionsEventType }}</h2>
        <div v-for="v in versions" :key="v.id" style="margin-bottom: 16px; padding: 16px; border: 1px solid #e5e7eb; border-radius: 8px;">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
            <div>
              <strong>v{{ v.version }}</strong>
              <span :class="['badge', v.is_compatible ? 'badge-active' : 'badge-failed']" style="margin-left: 8px;">
                {{ v.is_compatible ? '兼容' : '不兼容' }}
              </span>
              <span v-if="v.compatibility_note" style="margin-left: 8px; color: #6b7280; font-size: 12px;">{{ v.compatibility_note }}</span>
            </div>
            <span style="color: #9ca3af; font-size: 12px;">{{ formatTime(v.created_at) }}</span>
          </div>
          <pre style="background: #f9fafb; padding: 12px; border-radius: 6px; font-size: 12px; overflow-x: auto; max-height: 200px;">{{ formatJSON(v.schema_def) }}</pre>
        </div>
        <div v-if="versions.length >= 2" style="margin-top: 16px;">
          <h3 style="margin-bottom: 12px;">版本对比</h3>
          <div class="grid-2">
            <div>
              <select v-model="diffV1" class="form-group" style="width: 100%; padding: 8px; border: 1px solid #d1d5db; border-radius: 6px;">
                <option v-for="v in versions" :key="v.id" :value="v.version">v{{ v.version }}</option>
              </select>
            </div>
            <div>
              <select v-model="diffV2" class="form-group" style="width: 100%; padding: 8px; border: 1px solid #d1d5db; border-radius: 6px;">
                <option v-for="v in versions" :key="v.id" :value="v.version">v{{ v.version }}</option>
              </select>
            </div>
          </div>
          <button class="btn btn-outline btn-sm" style="margin-top: 12px;" @click="loadDiff">对比</button>
          <div v-if="diffData" class="grid-2" style="margin-top: 16px;">
            <div>
              <h4>v{{ diffV1 }}</h4>
              <pre style="background: #fef2f2; padding: 12px; border-radius: 6px; font-size: 12px; overflow-x: auto; max-height: 300px;">{{ formatJSON(diffData.version_1?.schema_def) }}</pre>
            </div>
            <div>
              <h4>v{{ diffV2 }}</h4>
              <pre style="background: #dcfce7; padding: 12px; border-radius: 6px; font-size: 12px; overflow-x: auto; max-height: 300px;">{{ formatJSON(diffData.version_2?.schema_def) }}</pre>
            </div>
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn btn-outline" @click="showVersionsModal = false">关闭</button>
        </div>
      </div>
    </div>

    <SchemaMigrationWizard
      v-model:visible="showMigrationWizard"
      :eventType="migrationEventType"
      :versions="migrationVersions"
      @refresh="fetchSchemas"
    />
  </div>
</template>

<script setup lang="ts">
const api = useApiStore()
const schemas = ref<any[]>([])
const showRegisterModal = ref(false)
const showVersionsModal = ref(false)
const versions = ref<any[]>([])
const versionsEventType = ref('')
const compatResult = ref<any>(null)
const diffV1 = ref(1)
const diffV2 = ref(2)
const diffData = ref<any>(null)
const showMigrationWizard = ref(false)
const migrationEventType = ref('')
const migrationVersions = ref<any[]>([])

const form = ref({
  event_type: '',
  schema_def: ''
})

const fetchSchemas = async () => {
  if (!api.tenantId) return
  try {
    schemas.value = await api.get('/schemas')
  } catch (e) {}
}

const viewVersions = async (eventType: string) => {
  versionsEventType.value = eventType
  try {
    versions.value = await api.get(`/schemas/${eventType}/versions`)
    if (versions.value.length >= 2) {
      diffV1.value = versions.value[versions.value.length - 1].version
      diffV2.value = versions.value[0].version
    }
    showVersionsModal.value = true
  } catch (e) {}
}

const checkCompat = async (eventType: string) => {
  const schemaDef = prompt('输入新的Schema定义(JSON):')
  if (!schemaDef) return
  try {
    compatResult.value = await api.post(`/schemas/${eventType}/check-compatibility`, { schema_def: schemaDef })
  } catch (e) {}
}

const checkFormCompat = async () => {
  try {
    compatResult.value = await api.post(`/schemas/${form.value.event_type}/check-compatibility`, { schema_def: form.value.schema_def })
  } catch (e) {}
}

const loadDiff = async () => {
  try {
    diffData.value = await api.get(`/schemas/${versionsEventType.value}/diff?v1=${diffV1.value}&v2=${diffV2.value}`)
  } catch (e) {}
}

const submitSchema = async () => {
  try {
    await api.post('/schemas', form.value)
    showRegisterModal.value = false
    compatResult.value = null
    form.value = { event_type: '', schema_def: '' }
    fetchSchemas()
  } catch (e) {}
}

const formatTime = (t: string) => {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN')
}

const formatJSON = (s: string) => {
  try {
    return JSON.stringify(JSON.parse(s), null, 2)
  } catch {
    return s
  }
}

const openMigrationWizard = async (eventType: string) => {
  migrationEventType.value = eventType
  try {
    migrationVersions.value = await api.get(`/schemas/${eventType}/versions`)
    showMigrationWizard.value = true
  } catch (e) {}
}

onMounted(fetchSchemas)
</script>
