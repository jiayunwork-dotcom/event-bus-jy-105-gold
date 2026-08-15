<template>
  <div class="migration-progress">
    <div class="progress-header">
      <div class="progress-status">
        <span class="status-indicator" :style="{ background: statusColor }"></span>
        <span class="status-text">{{ statusLabel }}</span>
      </div>
      <button
        v-if="showCancel && (progress.status === 'running' || progress.status === 'rollback_running')"
        class="btn btn-danger btn-sm"
        @click="handleCancel"
      >
        取消
      </button>
    </div>

    <div class="progress-bar-container">
      <div class="progress-bar">
        <div
          class="progress-bar-fill"
          :style="{ width: `${progress.progress_percent}%`, background: statusColor }"
        ></div>
      </div>
      <span class="progress-percent">{{ progress.progress_percent.toFixed(1) }}%</span>
    </div>

    <div class="progress-stats">
      <div class="stat-item">
        <span class="stat-label">总数</span>
        <span class="stat-value">{{ progress.total_events }}</span>
      </div>
      <div class="stat-item">
        <span class="stat-label">已处理</span>
        <span class="stat-value success">{{ progress.processed_events }}</span>
      </div>
      <div class="stat-item">
        <span class="stat-label">失败</span>
        <span class="stat-value failed">{{ progress.failed_events }}</span>
      </div>
      <div v-if="progress.estimated_remaining_seconds" class="stat-item">
        <span class="stat-label">预计剩余</span>
        <span class="stat-value">{{ formatTimeRemaining(progress.estimated_remaining_seconds) }}</span>
      </div>
    </div>

    <div v-if="progress.error_message" class="progress-error">
      <strong>错误:</strong> {{ progress.error_message }}
    </div>
  </div>
</template>

<script setup lang="ts">
import type { PropType } from 'vue'
import type { MigrationProgress } from '../types/migration'
import { MIGRATION_STATUS_LABELS, MIGRATION_STATUS_COLORS } from '../types/migration'

const props = defineProps({
  progress: { type: Object as PropType<MigrationProgress>, required: true },
  showCancel: { type: Boolean, default: true }
})

const emit = defineEmits<{
  cancel: []
}>()

const { formatTimeRemaining } = useMigration()

const statusLabel = computed(() => MIGRATION_STATUS_LABELS[props.progress.status as keyof typeof MIGRATION_STATUS_LABELS] || props.progress.status)
const statusColor = computed(() => MIGRATION_STATUS_COLORS[props.progress.status as keyof typeof MIGRATION_STATUS_COLORS] || '#6b7280')

const handleCancel = () => {
  emit('cancel')
}
</script>

<style scoped>
.migration-progress {
  background: #f9fafb;
  border-radius: 8px;
  padding: 16px;
}

.progress-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.progress-status {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-indicator {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.status-text {
  font-weight: 600;
  color: #374151;
}

.progress-bar-container {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.progress-bar {
  flex: 1;
  height: 10px;
  background: #e5e7eb;
  border-radius: 5px;
  overflow: hidden;
}

.progress-bar-fill {
  height: 100%;
  border-radius: 5px;
  transition: width 0.3s ease;
}

.progress-percent {
  font-size: 14px;
  font-weight: 600;
  color: #374151;
  min-width: 50px;
  text-align: right;
}

.progress-stats {
  display: flex;
  gap: 24px;
  flex-wrap: wrap;
}

.stat-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.stat-label {
  font-size: 12px;
  color: #6b7280;
}

.stat-value {
  font-size: 18px;
  font-weight: 600;
  color: #1f2937;
}

.stat-value.success {
  color: #10b981;
}

.stat-value.failed {
  color: #ef4444;
}

.progress-error {
  margin-top: 12px;
  padding: 12px;
  background: #fef2f2;
  border: 1px solid #fee2e2;
  border-radius: 6px;
  color: #991b1b;
  font-size: 13px;
}
</style>
