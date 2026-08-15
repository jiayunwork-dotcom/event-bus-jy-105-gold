<template>
  <div v-if="visible" class="modal-overlay" @click.self="handleClose">
    <div class="modal migration-wizard" style="max-width: 1000px; max-height: 90vh;">
      <div class="modal-header">
        <h2>Schema版本迁移向导</h2>
        <button class="modal-close" @click="handleClose">×</button>
      </div>

      <div class="wizard-steps">
        <div
          v-for="(step, index) in steps"
          :key="index"
          class="wizard-step"
          :class="{ active: currentStep === index, completed: currentStep > index }"
        >
          <div class="step-number">{{ index + 1 }}</div>
          <div class="step-label">{{ step }}</div>
        </div>
      </div>

      <div class="wizard-body">
        <div v-if="currentStep === 0" class="step-content">
          <h3>选择目标版本</h3>
          <p class="step-description">
            选择要迁移的源版本和目标版本。迁移将把指定事件类型下所有使用源版本Schema的历史事件转换为目标版本。
          </p>
          
          <div class="form-group">
            <label>事件类型</label>
            <input :value="eventType" disabled class="disabled" />
          </div>

          <div class="grid-2">
            <div class="form-group">
              <label>源版本</label>
              <select v-model="sourceVersion" @change="onVersionChange">
                <option v-for="v in availableVersions" :key="v.version" :value="v.version">
                  v{{ v.version }} {{ v.is_compatible ? '(兼容)' : '(不兼容)' }}
                </option>
              </select>
            </div>
            <div class="form-group">
              <label>目标版本</label>
              <select v-model="targetVersion" @change="onVersionChange">
                <option v-for="v in availableVersions" :key="v.version" :value="v.version">
                  v{{ v.version }} {{ v.is_compatible ? '(兼容)' : '(不兼容)' }}
                </option>
              </select>
            </div>
          </div>

          <div v-if="sourceVersion && targetVersion" class="version-info">
            <div class="grid-2">
              <div>
                <h4>源版本 Schema (v{{ sourceVersion }})</h4>
                <pre>{{ formatJSON(sourceSchemaDef) }}</pre>
              </div>
              <div>
                <h4>目标版本 Schema (v{{ targetVersion }})</h4>
                <pre>{{ formatJSON(targetSchemaDef) }}</pre>
              </div>
            </div>
          </div>
        </div>

        <div v-else-if="currentStep === 1" class="step-content">
          <MigrationRuleEditor
            v-model="migrationRules"
            :sourceVersion="sourceVersion"
            :targetVersion="targetVersion"
            :sourceSchemaDef="sourceSchemaDef"
            :targetSchemaDef="targetSchemaDef"
          />
        </div>

        <div v-else-if="currentStep === 2" class="step-content">
          <MigrationPreview
            :sourceVersion="sourceVersion"
            :targetVersion="targetVersion"
            :results="previewResults"
            :loading="previewLoading"
          />
        </div>

        <div v-else-if="currentStep === 3" class="step-content">
          <div v-if="!currentMigrationId">
            <h3>确认执行迁移</h3>
            <p class="step-description">
              请确认以下迁移信息无误。迁移开始后，系统将在后台异步执行，您可以随时查看进度或取消。
            </p>
            <div class="confirm-info">
              <div class="info-row">
                <span class="info-label">事件类型:</span>
                <span class="info-value">{{ eventType }}</span>
              </div>
              <div class="info-row">
                <span class="info-label">版本迁移:</span>
                <span class="info-value">v{{ sourceVersion }} → v{{ targetVersion }}</span>
              </div>
              <div class="info-row">
                <span class="info-label">迁移规则:</span>
                <span class="info-value">{{ migrationRules.length }} 条</span>
              </div>
            </div>
            <div class="warning-box">
              <strong>注意:</strong>
              <ul>
                <li>迁移过程中，新发布的事件不受影响</li>
                <li>单条事件转换失败会被跳过，不影响其他事件</li>
                <li>迁移完成后24小时内可以回滚</li>
                <li>迁移不会阻塞正常的事件发布和消费流程</li>
              </ul>
            </div>
          </div>

          <div v-else>
            <MigrationProgress
              :progress="currentProgress"
              @cancel="handleCancelMigration"
            />
            <div v-if="isMigrationFinished" class="migration-finished">
              <div v-if="currentProgress.status === 'completed'" class="success-message">
                ✓ 迁移完成！成功处理 {{ currentProgress.processed_events }} 条，失败 {{ currentProgress.failed_events }} 条
              </div>
              <div v-else-if="currentProgress.status === 'failed'" class="error-message">
                ✗ 迁移失败: {{ currentProgress.error_message }}
              </div>
              <div v-else-if="currentProgress.status === 'cancelled'" class="warning-message">
                ⚠ 迁移已取消
              </div>
            </div>
          </div>
        </div>

        <div v-else-if="currentStep === 4" class="step-content">
          <MigrationHistory
            :migrations="migrationHistory"
            :loading="historyLoading"
            :pendingRollbackId="pendingRollbackId"
            @rollback="handleRollback"
          />
        </div>
      </div>

      <div class="modal-actions">
        <button v-if="currentStep > 0 && !currentMigrationId" class="btn btn-outline" @click="prevStep">上一步</button>
        <button v-if="currentStep < 3 && !currentMigrationId" class="btn btn-outline" @click="skipToHistory">查看历史</button>
        <button v-if="currentStep < 2" class="btn btn-primary" :disabled="!canProceed" @click="nextStep">下一步</button>
        <button v-else-if="currentStep === 2" class="btn btn-primary" @click="nextStep">继续</button>
        <button v-else-if="currentStep === 3 && !currentMigrationId" class="btn btn-primary" @click="executeMigration">开始迁移</button>
        <button v-if="currentStep === 3 && isMigrationFinished" class="btn btn-primary" @click="closeAndRefresh">完成</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { PropType } from 'vue'
import type { MigrationRule, SchemaMigration, MigrationProgress, MigrationPreviewResult, MigrationRuleValidationError } from '../types/migration'

const props = defineProps({
  visible: { type: Boolean, default: false },
  eventType: { type: String, default: '' },
  versions: { type: Array as PropType<any[]>, default: () => [] }
})

const emit = defineEmits<{
  'update:visible': [value: boolean]
  'refresh': []
}>()

const api = useApiStore()
const {
  validateRules,
  previewMigration,
  startMigration,
  getMigrationProgress,
  cancelMigration,
  rollbackMigration,
  listMigrations
} = useMigration()

const steps = ['选择目标版本', '配置迁移规则', '预览确认', '执行迁移', '迁移历史']
const currentStep = ref(0)
const sourceVersion = ref<number>(0)
const targetVersion = ref<number>(0)
const migrationRules = ref<MigrationRule[]>([])
const validationErrors = ref<MigrationRuleValidationError[]>([])
const previewResults = ref<MigrationPreviewResult[]>([])
const previewLoading = ref(false)
const currentMigrationId = ref('')
const currentProgress = ref<MigrationProgress | null>(null)
const migrationHistory = ref<SchemaMigration[]>([])
const historyLoading = ref(false)
const pendingRollbackId = ref('')
let progressPollingInterval: ReturnType<typeof setInterval> | null = null

const availableVersions = computed(() => props.versions.sort((a, b) => b.version - a.version))

const sourceSchemaDef = computed(() => {
  const v = props.versions.find(v => v.version === sourceVersion.value)
  return v?.schema_def || '{}'
})

const targetSchemaDef = computed(() => {
  const v = props.versions.find(v => v.version === targetVersion.value)
  return v?.schema_def || '{}'
})

const canProceed = computed(() => {
  if (currentStep.value === 0) {
    return sourceVersion.value > 0 && targetVersion.value > 0 && targetVersion.value > sourceVersion.value
  }
  if (currentStep.value === 1) {
    return migrationRules.value.length > 0 && validationErrors.value.length === 0
  }
  return true
})

const isMigrationFinished = computed(() => {
  if (!currentProgress.value) return false
  return ['completed', 'failed', 'cancelled', 'rollbacked', 'rollback_failed'].includes(currentProgress.value.status)
})

const onVersionChange = async () => {
  if (sourceVersion.value > 0 && targetVersion.value > 0) {
    if (targetVersion.value <= sourceVersion.value) {
      targetVersion.value = sourceVersion.value + 1
    }
  }
}

const validateCurrentRules = async () => {
  if (migrationRules.value.length === 0) {
    validationErrors.value = []
    return
  }
  try {
    const result = await validateRules(migrationRules.value)
    if (result && !result.valid) {
      validationErrors.value = result.errors || []
    } else {
      validationErrors.value = []
    }
  } catch {
    validationErrors.value = []
  }
}

const loadPreview = async () => {
  if (sourceVersion.value === 0 || targetVersion.value === 0 || migrationRules.value.length === 0) {
    previewResults.value = []
    return
  }

  previewLoading.value = true
  try {
    previewResults.value = await previewMigration(
      props.eventType,
      sourceVersion.value,
      targetVersion.value,
      migrationRules.value
    )
  } catch (e: any) {
    previewResults.value = []
  } finally {
    previewLoading.value = false
  }
}

const loadHistory = async () => {
  historyLoading.value = true
  try {
    migrationHistory.value = await listMigrations(props.eventType)
  } catch {
    migrationHistory.value = []
  } finally {
    historyLoading.value = false
  }
}

const nextStep = async () => {
  if (currentStep.value === 1) {
    await loadPreview()
  }
  if (currentStep.value < steps.length - 1) {
    currentStep.value++
  }
}

const prevStep = () => {
  if (currentStep.value > 0) {
    currentStep.value--
  }
}

const skipToHistory = () => {
  currentStep.value = 4
  loadHistory()
}

const executeMigration = async () => {
  try {
    const migration = await startMigration(
      props.eventType,
      sourceVersion.value,
      targetVersion.value,
      migrationRules.value
    )
    currentMigrationId.value = migration.id
    startProgressPolling()
  } catch (e: any) {
    alert(e.message || '启动迁移失败')
  }
}

const startProgressPolling = () => {
  const poll = async () => {
    if (!currentMigrationId.value) return
    try {
      currentProgress.value = await getMigrationProgress(currentMigrationId.value)
      if (isMigrationFinished.value) {
        stopProgressPolling()
      }
    } catch {
      // ignore
    }
  }
  poll()
  progressPollingInterval = setInterval(poll, 2000)
}

const stopProgressPolling = () => {
  if (progressPollingInterval) {
    clearInterval(progressPollingInterval)
    progressPollingInterval = null
  }
}

const handleCancelMigration = async () => {
  if (!confirm('确定要取消当前迁移吗？')) return
  try {
    await cancelMigration(currentMigrationId.value)
  } catch (e: any) {
    alert(e.message || '取消失败')
  }
}

const handleRollback = async (migrationId: string) => {
  pendingRollbackId.value = migrationId
  try {
    await rollbackMigration(migrationId)
    currentMigrationId.value = migrationId
    currentStep.value = 3
    startProgressPolling()
  } catch (e: any) {
    alert(e.message || '启动回滚失败')
  } finally {
    pendingRollbackId.value = ''
  }
}

const handleClose = () => {
  if (currentMigrationId.value && !isMigrationFinished.value) {
    if (!confirm('迁移正在进行中，确定要关闭吗？您可以稍后在历史记录中查看进度。')) {
      return
    }
  }
  stopProgressPolling()
  emit('update:visible', false)
}

const closeAndRefresh = () => {
  stopProgressPolling()
  emit('update:visible', false)
  emit('refresh')
}

const formatJSON = (s: string) => {
  try {
    return JSON.stringify(JSON.parse(s), null, 2)
  } catch {
    return s
  }
}

watch(() => migrationRules.value, validateCurrentRules, { deep: true })

watch(() => props.visible, (val) => {
  if (val && props.versions.length >= 2) {
    const sorted = [...props.versions].sort((a, b) => a.version - b.version)
    sourceVersion.value = sorted[0].version
    targetVersion.value = sorted[sorted.length - 1].version
  }
  if (val) {
    currentStep.value = 0
    migrationRules.value = []
    currentMigrationId.value = ''
    currentProgress.value = null
    loadHistory()
  }
})
</script>

<style scoped>
.migration-wizard {
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 24px;
  border-bottom: 1px solid #e5e7eb;
  flex-shrink: 0;
}

.modal-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}

.modal-close {
  background: none;
  border: none;
  font-size: 24px;
  cursor: pointer;
  color: #6b7280;
  padding: 0 8px;
}

.wizard-steps {
  display: flex;
  padding: 16px 24px;
  border-bottom: 1px solid #e5e7eb;
  background: #f9fafb;
  flex-shrink: 0;
  gap: 8px;
}

.wizard-step {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  border-radius: 20px;
  background: white;
  border: 1px solid #e5e7eb;
  cursor: pointer;
  transition: all 0.2s;
}

.wizard-step.active {
  background: #3b82f6;
  border-color: #3b82f6;
  color: white;
}

.wizard-step.completed {
  background: #d1fae5;
  border-color: #10b981;
  color: #065f46;
}

.step-number {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: #e5e7eb;
  color: #6b7280;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 600;
}

.wizard-step.active .step-number {
  background: white;
  color: #3b82f6;
}

.wizard-step.completed .step-number {
  background: #10b981;
  color: white;
}

.step-label {
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
}

.wizard-body {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}

.step-content h3 {
  margin: 0 0 8px 0;
  font-size: 18px;
  font-weight: 600;
}

.step-description {
  color: #6b7280;
  margin-bottom: 20px;
  font-size: 14px;
}

.disabled {
  background: #f3f4f6;
  color: #6b7280;
  cursor: not-allowed;
}

.version-info {
  margin-top: 20px;
}

.version-info h4 {
  margin: 0 0 8px 0;
  font-size: 14px;
  font-weight: 600;
  color: #374151;
}

.version-info pre {
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  padding: 12px;
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 11px;
  line-height: 1.5;
  max-height: 250px;
  overflow: auto;
}

.confirm-info {
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 20px;
}

.info-row {
  display: flex;
  justify-content: space-between;
  padding: 8px 0;
  border-bottom: 1px solid #e5e7eb;
}

.info-row:last-child {
  border-bottom: none;
}

.info-label {
  font-weight: 500;
  color: #374151;
}

.info-value {
  color: #1f2937;
  font-weight: 600;
}

.warning-box {
  background: #fffbeb;
  border: 1px solid #fcd34d;
  border-radius: 8px;
  padding: 16px 20px;
  color: #92400e;
}

.warning-box ul {
  margin: 8px 0 0 0;
  padding-left: 20px;
}

.warning-box li {
  margin-bottom: 4px;
}

.migration-finished {
  margin-top: 16px;
  padding: 16px;
  border-radius: 8px;
  text-align: center;
  font-weight: 600;
}

.success-message {
  background: #d1fae5;
  color: #065f46;
}

.error-message {
  background: #fee2e2;
  color: #991b1b;
}

.warning-message {
  background: #fef3c7;
  color: #92400e;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 16px 24px;
  border-top: 1px solid #e5e7eb;
  flex-shrink: 0;
}

.grid-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
</style>
