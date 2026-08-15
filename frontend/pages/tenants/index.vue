<template>
  <div>
    <div class="page-header">
      <h1>租户管理</h1>
      <button class="btn btn-primary" @click="showCreateModal = true">+ 创建租户</button>
    </div>

    <div class="card">
      <table>
        <thead>
          <tr>
            <th>租户ID</th>
            <th>名称</th>
            <th>状态</th>
            <th>发布QPS上限</th>
            <th>订阅数上限</th>
            <th>存储容量(MB)</th>
            <th>保留策略</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="tenant in tenants" :key="tenant.id">
            <td style="font-family: monospace; font-size: 12px;">{{ tenant.id.substring(0, 8) }}...</td>
            <td>{{ tenant.name }}</td>
            <td><span :class="['badge', `badge-${tenant.status}`]">{{ tenant.status }}</span></td>
            <td>{{ tenant.max_publish_qps }}</td>
            <td>{{ tenant.max_subscriptions }}</td>
            <td>{{ tenant.max_storage_mb }}</td>
            <td>{{ formatRetentionPolicy(tenant.retention_policy) }}</td>
            <td>
              <button class="btn btn-outline btn-sm" @click="editTenant(tenant)">编辑</button>
              <button v-if="tenant.status === 'active'" class="btn btn-danger btn-sm" style="margin-left: 8px;" @click="disableTenant(tenant.id)">禁用</button>
            </td>
          </tr>
          <tr v-if="tenants.length === 0">
            <td colspan="8" style="text-align: center; color: #9ca3af; padding: 40px;">暂无租户数据</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="showCreateModal" class="modal-overlay" @click.self="showCreateModal = false">
      <div class="modal">
        <h2>{{ editingTenant ? '编辑租户' : '创建租户' }}</h2>
        <div class="form-group">
          <label>租户名称</label>
          <input v-model="form.name" placeholder="输入租户名称" />
        </div>
        <div class="grid-2">
          <div class="form-group">
            <label>发布QPS上限</label>
            <input v-model.number="form.max_publish_qps" type="number" />
          </div>
          <div class="form-group">
            <label>订阅数上限</label>
            <input v-model.number="form.max_subscriptions" type="number" />
          </div>
        </div>
        <div class="form-group">
          <label>存储容量(MB)</label>
          <input v-model.number="form.max_storage_mb" type="number" />
        </div>
        <div class="form-group">
          <label>保留策略类型</label>
          <select v-model="form.retention_policy.type">
            <option value="time">按时间</option>
            <option value="capacity">按容量</option>
          </select>
        </div>
        <div class="form-group" v-if="form.retention_policy.type === 'time'">
          <label>保留天数</label>
          <select v-model.number="form.retention_policy.value_days">
            <option :value="7">7天</option>
            <option :value="30">30天</option>
            <option :value="0">永久</option>
          </select>
        </div>
        <div class="form-group" v-if="form.retention_policy.type === 'capacity'">
          <label>容量上限(MB)</label>
          <input v-model.number="form.retention_policy.value_mb" type="number" />
        </div>
        <div class="modal-actions">
          <button class="btn btn-outline" @click="closeModal">取消</button>
          <button class="btn btn-primary" @click="submitForm">{{ editingTenant ? '保存' : '创建' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
const tenants = ref<any[]>([])
const showCreateModal = ref(false)
const editingTenant = ref<any>(null)
const form = ref({
  name: '',
  max_publish_qps: 1000,
  max_subscriptions: 100,
  max_storage_mb: 10240,
  retention_policy: { type: 'time', value_days: 30, value_mb: 0 }
})

const ADMIN_HEADERS = {
  'X-Admin-Key': 'eventbus-admin-key-2024',
  'Content-Type': 'application/json'
}

const fetchTenants = async () => {
  try {
    const res = await fetch('/api/tenants', { headers: ADMIN_HEADERS })
    if (res.ok) tenants.value = await res.json()
  } catch (e) {}
}

const editTenant = (tenant: any) => {
  editingTenant.value = tenant
  form.value = {
    name: tenant.name,
    max_publish_qps: tenant.max_publish_qps,
    max_subscriptions: tenant.max_subscriptions,
    max_storage_mb: tenant.max_storage_mb,
    retention_policy: tenant.retention_policy || { type: 'time', value_days: 30 }
  }
  showCreateModal.value = true
}

const disableTenant = async (id: string) => {
  try {
    await fetch(`/api/tenants/${id}/disable`, { method: 'PUT', headers: ADMIN_HEADERS })
    fetchTenants()
  } catch (e) {}
}

const submitForm = async () => {
  try {
    if (editingTenant.value) {
      await fetch(`/api/tenants/${editingTenant.value.id}`, {
        method: 'PUT',
        headers: ADMIN_HEADERS,
        body: JSON.stringify(form.value)
      })
    } else {
      await fetch('/api/tenants', {
        method: 'POST',
        headers: ADMIN_HEADERS,
        body: JSON.stringify(form.value)
      })
    }
    closeModal()
    fetchTenants()
    refreshSidebarTenants()
  } catch (e) {}
}

const refreshSidebarTenants = () => {
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent('refresh-tenants'))
  }
}

const closeModal = () => {
  showCreateModal.value = false
  editingTenant.value = null
  form.value = {
    name: '',
    max_publish_qps: 1000,
    max_subscriptions: 100,
    max_storage_mb: 10240,
    retention_policy: { type: 'time', value_days: 30, value_mb: 0 }
  }
}

const formatRetentionPolicy = (policy: any) => {
  if (!policy) return '-'
  if (policy.type === 'time') {
    return policy.value_days === 0 ? '永久' : `${policy.value_days}天`
  }
  return `${policy.value_mb}MB`
}

onMounted(fetchTenants)
</script>
