<template>
  <div class="app-layout">
    <aside class="sidebar">
      <div class="logo">
        <span class="logo-icon">⚡</span> EventBus
      </div>
      <nav class="nav-section">
        <div class="nav-title">Core</div>
        <NuxtLink to="/tenants" class="nav-item" :class="{ active: isActive('/tenants') }">
          <span class="nav-icon">🏢</span> 租户管理
        </NuxtLink>
        <NuxtLink to="/schemas" class="nav-item" :class="{ active: isActive('/schemas') }">
          <span class="nav-icon">📋</span> Schema管理
        </NuxtLink>
        <NuxtLink to="/subscriptions" class="nav-item" :class="{ active: isActive('/subscriptions') }">
          <span class="nav-icon">🔔</span> 订阅与编排
        </NuxtLink>
      </nav>
      <nav class="nav-section">
        <div class="nav-title">Operations</div>
        <NuxtLink to="/monitor" class="nav-item" :class="{ active: isActive('/monitor') }">
          <span class="nav-icon">📊</span> 监控Dashboard
        </NuxtLink>
        <NuxtLink to="/traces" class="nav-item" :class="{ active: isActive('/traces') }">
          <span class="nav-icon">🔍</span> 事件追踪
        </NuxtLink>
        <NuxtLink to="/dead-letter" class="nav-item" :class="{ active: isActive('/dead-letter') }">
          <span class="nav-icon">💀</span> 死信队列
        </NuxtLink>
      </nav>
      <div class="tenant-selector">
        <label style="color: #9ca3af; font-size: 11px; text-transform: uppercase; display: block; margin-bottom: 6px;">当前租户</label>
        <select v-model="currentTenant" @change="switchTenant">
          <option value="">选择租户...</option>
          <option v-for="t in tenants" :key="t.id" :value="t.id">{{ t.name }}</option>
        </select>
      </div>
    </aside>
    <main class="main-content">
      <slot />
    </main>
  </div>
</template>

<script setup lang="ts">
const route = useRoute()
const api = useApiStore()
const tenants = ref<any[]>([])
const currentTenant = ref('')

const isActive = (path: string) => route.path.startsWith(path)

const switchTenant = () => {
  api.setTenant(currentTenant.value)
}

const loadTenants = async () => {
  try {
    const res = await fetch('/api/tenants', {
      headers: {
        'X-Admin-Key': 'eventbus-admin-key-2024',
        'Content-Type': 'application/json'
      }
    })
    if (res.ok) tenants.value = await res.json()
  } catch (e) {}
}

onMounted(async () => {
  loadTenants()
  if (typeof window !== 'undefined') {
    window.addEventListener('refresh-tenants', loadTenants)
  }
})

onUnmounted(() => {
  if (typeof window !== 'undefined') {
    window.removeEventListener('refresh-tenants', loadTenants)
  }
})
</script>
