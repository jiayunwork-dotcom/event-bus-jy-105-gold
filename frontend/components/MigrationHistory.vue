<template>
  <div class="migration-history">
    <h3>迁移历史记录</h3>
    <div v-if="loading" class="loading">
      正在加载历史记录...
    </div>
    <div v-else-if="migrations.length === 0" class="empty-state">
      暂无迁移记录
    </div>
    <div v-else class="history-list">
      <div
        v-for="migration in migrations"
        :key="migration.id"
        class="history-item"
      >
        <div class="history-header">
          <div class="history-version">
            v{{ migration.source_version }} → v{{ migration.target_version }}
          </div>
          <div class="history-header-actions">
            <span
              class="status-badge"
              :style="{ background: getStatusColor(migration.status), color: 'white' }"
            >
              {{ getStatusLabel(migration.status) }}
            </span>
            <button
              v-if="migration.status === 'completed' || migration.status === 'rollbacked'"
              class="btn btn-info btn-sm"
              :disabled="!!impactLoadingMap[migration.id]"
              @click="handleAnalyzeImpact(migration.id)"
            >
              {{ impactLoadingMap[migration.id] ? '分析中...' : '影响分析' }}
            </button>
          </div>
        </div>
        <div class="history-stats">
          <div class="stat">
            <span class="stat-label">总数</span>
            <span class="stat-value">{{ migration.total_events }}</span>
          </div>
          <div class="stat">
            <span class="stat-label">成功</span>
            <span class="stat-value success">{{ migration.processed_events }}</span>
          </div>
          <div class="stat">
            <span class="stat-label">失败</span>
            <span class="stat-value failed">{{ migration.failed_events }}</span>
          </div>
        </div>
        <div class="history-time">
          <span>创建: {{ formatTime(migration.created_at) }}</span>
          <span v-if="migration.completed_at">完成: {{ formatTime(migration.completed_at) }}</span>
        </div>
        <div v-if="migration.error_message" class="history-error">
          错误: {{ migration.error_message }}
        </div>
        <div v-if="canRollback(migration)" class="history-actions">
          <button
            class="btn btn-warning btn-sm"
            :disabled="!!pendingRollbackId"
            @click="handleRollback(migration.id)"
          >
            {{ pendingRollbackId === migration.id ? '回滚中...' : '回滚' }}
          </button>
          <span v-if="migration.rollback_deadline" class="rollback-deadline">
            回滚截止: {{ formatTime(migration.rollback_deadline) }}
          </span>
        </div>

        <div v-if="impactReports[migration.id]" class="impact-panel">
          <div class="impact-panel-header">
            <span class="impact-panel-title">迁移影响分析报告</span>
            <button class="impact-close-btn" @click="closeImpactPanel(migration.id)">×</button>
          </div>

          <div v-if="!impactReports[migration.id].has_impact" class="impact-no-impact">
            <span class="no-impact-badge">✓ 无影响</span>
            <span class="no-impact-text">本次迁移不影响任何订阅或编排配置</span>
          </div>

          <div v-else>
            <div class="impact-section">
              <h4>被修改的字段</h4>
              <div class="affected-fields">
                <span
                  v-for="field in impactReports[migration.id].affected_fields"
                  :key="field"
                  class="field-tag"
                >{{ field }}</span>
              </div>
            </div>

            <div v-if="impactReports[migration.id].affected_subscriptions.length > 0" class="impact-section">
              <h4>受影响的订阅 <span class="impact-count">{{ impactReports[migration.id].affected_subscriptions.length }}</span></h4>
              <div class="impact-list">
                <div
                  v-for="sub in impactReports[migration.id].affected_subscriptions"
                  :key="sub.id"
                  class="impact-item impacted"
                  @click="navigateToSubscription(sub.id)"
                >
                  <div class="impact-item-header">
                    <span class="impact-item-name">{{ sub.name }}</span>
                    <span class="impact-item-type">订阅</span>
                  </div>
                  <div class="impact-item-detail">
                    <span class="impact-label">过滤表达式:</span>
                    <code class="impact-code">{{ sub.filter_expression }}</code>
                  </div>
                  <div class="impact-item-detail">
                    <span class="impact-label">引用字段:</span>
                    <span
                      v-for="f in sub.matched_fields"
                      :key="f"
                      class="matched-field-tag"
                    >{{ f }}</span>
                  </div>
                </div>
              </div>
            </div>

            <div v-if="impactReports[migration.id].affected_dag_nodes.length > 0" class="impact-section">
              <h4>受影响的编排DAG节点 <span class="impact-count">{{ impactReports[migration.id].affected_dag_nodes.length }}</span></h4>
              <div class="impact-list">
                <div
                  v-for="node in impactReports[migration.id].affected_dag_nodes"
                  :key="`${node.dag_id}-${node.node_id}`"
                  class="impact-item impacted"
                  @click="navigateToDAG(node.subscription_id)"
                >
                  <div class="impact-item-header">
                    <span class="impact-item-name">{{ node.node_name }}</span>
                    <span class="impact-item-type">{{ node.node_type }} 节点</span>
                  </div>
                  <div class="impact-item-detail">
                    <span class="impact-label">所属订阅:</span>
                    <span>{{ node.subscription_name }}</span>
                  </div>
                  <div class="impact-item-detail">
                    <span class="impact-label">引用字段:</span>
                    <span
                      v-for="f in node.matched_fields"
                      :key="f"
                      class="matched-field-tag"
                    >{{ f }}</span>
                  </div>
                </div>
              </div>
            </div>

            <div v-if="impactReports[migration.id].suggestions.length > 0" class="impact-section">
              <h4>建议操作</h4>
              <ul class="suggestion-list">
                <li
                  v-for="(suggestion, idx) in impactReports[migration.id].suggestions"
                  :key="idx"
                >{{ suggestion }}</li>
              </ul>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { PropType } from 'vue'
import type { SchemaMigration, MigrationImpactReport } from '../types/migration'
import { MIGRATION_STATUS_LABELS, MIGRATION_STATUS_COLORS } from '../types/migration'

defineProps({
  migrations: { type: Array as PropType<SchemaMigration[]>, default: () => [] },
  loading: { type: Boolean, default: false },
  pendingRollbackId: { type: String, default: '' }
})

const emit = defineEmits<{
  rollback: [migrationId: string]
}>()

const { analyzeImpact } = useMigration()

const impactReports = ref<Record<string, MigrationImpactReport>>({})
const impactLoadingMap = ref<Record<string, boolean>>({})

const getStatusLabel = (status: string) => {
  return MIGRATION_STATUS_LABELS[status as keyof typeof MIGRATION_STATUS_LABELS] || status
}

const getStatusColor = (status: string) => {
  return MIGRATION_STATUS_COLORS[status as keyof typeof MIGRATION_STATUS_COLORS] || '#6b7280'
}

const canRollback = (migration: SchemaMigration) => {
  if (migration.status !== 'completed') return false
  if (!migration.rollback_deadline) return false
  return new Date(migration.rollback_deadline) > new Date()
}

const formatTime = (t: string) => {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN')
}

const handleRollback = (migrationId: string) => {
  if (confirm('确定要回滚此迁移吗？回滚将恢复所有事件的原始数据。')) {
    emit('rollback', migrationId)
  }
}

const handleAnalyzeImpact = async (migrationId: string) => {
  impactLoadingMap.value[migrationId] = true
  try {
    const report = await analyzeImpact(migrationId)
    impactReports.value[migrationId] = report
  } catch (e: any) {
    alert(e.message || '影响分析失败')
  } finally {
    impactLoadingMap.value[migrationId] = false
  }
}

const closeImpactPanel = (migrationId: string) => {
  delete impactReports.value[migrationId]
}

const navigateToSubscription = (subscriptionId: string) => {
  window.location.hash = `/subscriptions?edit=${subscriptionId}`
}

const navigateToDAG = (subscriptionId: string) => {
  window.location.hash = `/subscriptions?dag=${subscriptionId}`
}
</script>

<style scoped>
.migration-history {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.migration-history h3 {
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

.history-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: 600px;
  overflow-y: auto;
}

.history-item {
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 16px;
}

.history-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.history-version {
  font-size: 16px;
  font-weight: 600;
  color: #1f2937;
}

.history-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-badge {
  font-size: 12px;
  font-weight: 600;
  padding: 4px 10px;
  border-radius: 12px;
}

.history-stats {
  display: flex;
  gap: 24px;
  margin-bottom: 8px;
}

.stat {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.stat-label {
  font-size: 11px;
  color: #6b7280;
}

.stat-value {
  font-size: 16px;
  font-weight: 600;
  color: #1f2937;
}

.stat-value.success {
  color: #10b981;
}

.stat-value.failed {
  color: #ef4444;
}

.history-time {
  display: flex;
  gap: 16px;
  font-size: 12px;
  color: #6b7280;
  margin-bottom: 8px;
}

.history-error {
  padding: 8px 12px;
  background: #fef2f2;
  border-radius: 4px;
  color: #991b1b;
  font-size: 12px;
  margin-bottom: 8px;
}

.history-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-top: 8px;
  border-top: 1px solid #e5e7eb;
}

.rollback-deadline {
  font-size: 12px;
  color: #6b7280;
}

.btn-warning {
  background: #f59e0b;
  color: white;
  border: none;
  padding: 6px 16px;
  border-radius: 6px;
  cursor: pointer;
  font-weight: 500;
}

.btn-warning:hover:not(:disabled) {
  background: #d97706;
}

.btn-warning:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-info {
  background: #3b82f6;
  color: white;
  border: none;
  padding: 6px 16px;
  border-radius: 6px;
  cursor: pointer;
  font-weight: 500;
  font-size: 12px;
}

.btn-info:hover:not(:disabled) {
  background: #2563eb;
}

.btn-info:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.impact-panel {
  margin-top: 12px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: white;
  overflow: hidden;
}

.impact-panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 16px;
  background: #f0f9ff;
  border-bottom: 1px solid #e5e7eb;
}

.impact-panel-title {
  font-size: 14px;
  font-weight: 600;
  color: #1e40af;
}

.impact-close-btn {
  background: none;
  border: none;
  font-size: 18px;
  cursor: pointer;
  color: #6b7280;
  padding: 0 4px;
  line-height: 1;
}

.impact-close-btn:hover {
  color: #374151;
}

.impact-no-impact {
  padding: 20px;
  text-align: center;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.no-impact-badge {
  display: inline-block;
  background: #d1fae5;
  color: #065f46;
  padding: 4px 14px;
  border-radius: 12px;
  font-weight: 600;
  font-size: 13px;
}

.no-impact-text {
  color: #6b7280;
  font-size: 13px;
}

.impact-section {
  padding: 12px 16px;
  border-bottom: 1px solid #f3f4f6;
}

.impact-section:last-child {
  border-bottom: none;
}

.impact-section h4 {
  margin: 0 0 8px 0;
  font-size: 13px;
  font-weight: 600;
  color: #374151;
}

.impact-count {
  display: inline-block;
  background: #ef4444;
  color: white;
  font-size: 11px;
  padding: 1px 8px;
  border-radius: 10px;
  font-weight: 600;
  margin-left: 4px;
}

.affected-fields {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.field-tag {
  background: #fef3c7;
  color: #92400e;
  padding: 3px 10px;
  border-radius: 4px;
  font-size: 12px;
  font-family: 'Monaco', 'Menlo', monospace;
  border: 1px solid #fcd34d;
}

.impact-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.impact-item {
  padding: 10px 12px;
  border-radius: 6px;
  border: 1px solid #e5e7eb;
  background: #f9fafb;
  cursor: pointer;
  transition: all 0.15s;
}

.impact-item:hover {
  box-shadow: 0 2px 6px rgba(0,0,0,0.08);
}

.impact-item.impacted {
  border-left: 3px solid #ef4444;
  background: #fef2f2;
}

.impact-item.impacted:hover {
  background: #fee2e2;
}

.impact-item-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.impact-item-name {
  font-weight: 600;
  font-size: 13px;
  color: #1f2937;
}

.impact-item-type {
  font-size: 11px;
  color: #6b7280;
  background: #e5e7eb;
  padding: 2px 8px;
  border-radius: 4px;
}

.impact-item-detail {
  font-size: 12px;
  color: #4b5563;
  margin-top: 4px;
  display: flex;
  align-items: flex-start;
  gap: 6px;
  flex-wrap: wrap;
}

.impact-label {
  color: #6b7280;
  white-space: nowrap;
}

.impact-code {
  background: #f3f4f6;
  border: 1px solid #e5e7eb;
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 11px;
  font-family: 'Monaco', 'Menlo', monospace;
  max-width: 300px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: inline-block;
}

.matched-field-tag {
  background: #fee2e2;
  color: #991b1b;
  padding: 1px 8px;
  border-radius: 3px;
  font-size: 11px;
  font-family: 'Monaco', 'Menlo', monospace;
  border: 1px solid #fca5a5;
}

.suggestion-list {
  margin: 0;
  padding-left: 20px;
  font-size: 13px;
  color: #374151;
}

.suggestion-list li {
  margin-bottom: 6px;
  line-height: 1.5;
}
</style>
