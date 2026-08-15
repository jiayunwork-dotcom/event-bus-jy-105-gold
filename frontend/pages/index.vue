<template>
  <div class="page-header">
    <h1>事件总线控制台</h1>
  </div>
  <div class="grid-4">
    <div class="stat-card">
      <div class="stat-value">{{ stats.totalEvents || 0 }}</div>
      <div class="stat-label">总事件数</div>
    </div>
    <div class="stat-card">
      <div class="stat-value">{{ stats.activeSubscriptions || 0 }}</div>
      <div class="stat-label">活跃订阅</div>
    </div>
    <div class="stat-card">
      <div class="stat-value">{{ stats.dlqDepth || 0 }}</div>
      <div class="stat-label">死信队列深度</div>
    </div>
    <div class="stat-card">
      <div class="stat-value">{{ stats.avgLatency || 0 }}ms</div>
      <div class="stat-label">平均延迟</div>
    </div>
  </div>
</template>

<script setup lang="ts">
const stats = ref<any>({})

onMounted(async () => {
  try {
    const api = useApiStore()
    if (api.tenantId) {
      stats.value = await api.get('/monitor/dashboard')
    }
  } catch (e) {}
})
</script>
