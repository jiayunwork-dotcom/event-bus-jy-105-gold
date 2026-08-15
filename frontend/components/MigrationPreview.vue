<template>
  <div class="migration-preview">
    <h3>迁移预览 - 10条样本数据转换对比</h3>
    <div v-if="loading" class="loading">
      正在加载预览数据...
    </div>
    <div v-else-if="results.length === 0" class="empty-state">
      暂无预览数据
    </div>
    <div v-else class="preview-list">
      <div
        v-for="(result, index) in results"
        :key="result.event_id || index"
        class="preview-item"
        :class="{ 'has-error': !result.success }"
      >
        <div class="preview-header">
          <span class="preview-index">样本 {{ index + 1 }}</span>
          <span class="event-id">ID: {{ result.event_id }}</span>
          <span :class="['preview-status', result.success ? 'success' : 'failed']">
            {{ result.success ? '转换成功' : '转换失败' }}
          </span>
        </div>
        <div v-if="!result.success && result.error_message" class="preview-error">
          {{ result.error_message }}
        </div>
        <div class="preview-content">
          <div class="preview-column">
            <h4>转换前 (v{{ sourceVersion }})</h4>
            <pre>{{ formatJSON(result.original_payload) }}</pre>
          </div>
          <div class="preview-arrow">→</div>
          <div class="preview-column">
            <h4>转换后 (v{{ targetVersion }})</h4>
            <pre>{{ formatJSON(result.converted_payload) }}</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { PropType } from 'vue'
import type { MigrationPreviewResult } from '../types/migration'

defineProps({
  sourceVersion: { type: Number, required: true },
  targetVersion: { type: Number, required: true },
  results: { type: Array as PropType<MigrationPreviewResult[]>, default: () => [] },
  loading: { type: Boolean, default: false }
})

const formatJSON = (obj: any) => {
  try {
    return JSON.stringify(obj, null, 2)
  } catch {
    return String(obj)
  }
}
</script>

<style scoped>
.migration-preview {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.migration-preview h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #374151;
}

.loading, .empty-state {
  padding: 40px;
  text-align: center;
  color: #9ca3af;
  font-size: 14px;
}

.preview-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: 400px;
  overflow-y: auto;
}

.preview-item {
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  overflow: hidden;
}

.preview-item.has-error {
  border-color: #fecaca;
}

.preview-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  background: #f3f4f6;
  border-bottom: 1px solid #e5e7eb;
}

.preview-index {
  font-weight: 600;
  color: #374151;
}

.event-id {
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 12px;
  color: #6b7280;
}

.preview-status {
  margin-left: auto;
  font-size: 12px;
  font-weight: 600;
  padding: 2px 10px;
  border-radius: 12px;
}

.preview-status.success {
  background: #d1fae5;
  color: #065f46;
}

.preview-status.failed {
  background: #fee2e2;
  color: #991b1b;
}

.preview-error {
  padding: 10px 16px;
  background: #fef2f2;
  color: #991b1b;
  font-size: 13px;
  border-bottom: 1px solid #fee2e2;
}

.preview-content {
  display: flex;
  align-items: stretch;
  gap: 0;
}

.preview-column {
  flex: 1;
  padding: 12px 16px;
  min-width: 0;
}

.preview-column h4 {
  margin: 0 0 8px 0;
  font-size: 13px;
  font-weight: 600;
  color: #374151;
}

.preview-column pre {
  margin: 0;
  padding: 12px;
  background: white;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 11px;
  line-height: 1.5;
  max-height: 200px;
  overflow-x: auto;
  overflow-y: auto;
}

.preview-arrow {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  font-size: 24px;
  color: #9ca3af;
  background: #f3f4f6;
}
</style>
