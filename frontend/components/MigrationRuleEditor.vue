<template>
  <div class="rule-editor">
    <div class="rule-editor-header">
      <h3>配置迁移规则</h3>
      <div class="rule-editor-actions">
        <button class="btn btn-outline btn-sm" @click="addRule('rename')">+ 重命名</button>
        <button class="btn btn-outline btn-sm" @click="addRule('delete')">+ 删除</button>
        <button class="btn btn-outline btn-sm" @click="addRule('add')">+ 新增</button>
        <button class="btn btn-outline btn-sm" @click="addRule('convert')">+ 转换</button>
        <button class="btn btn-outline btn-sm" @click="addRule('map_path')">+ 映射</button>
      </div>
    </div>

    <div class="rule-editor-body">
      <div class="fields-panel">
        <div class="fields-column">
          <h4>源版本字段 (v{{ sourceVersion }})</h4>
          <div class="fields-list">
            <div
              v-for="field in sourceFields"
              :key="field.path"
              class="field-item"
              :class="{ 'dragging': dragSource === field.path, 'mapped': isFieldMapped(field.path, 'source') }"
              draggable="true"
              @dragstart="onDragStart($event, field.path, 'source')"
              @dragover.prevent
              @drop="onDrop($event, field.path, 'source')"
            >
              <span class="field-name">{{ field.path }}</span>
              <span class="field-type">{{ field.type }}</span>
              <span v-if="field.required" class="field-required">*</span>
            </div>
            <div v-if="sourceFields.length === 0" class="empty-state">
              暂无字段
            </div>
          </div>
        </div>

        <div class="mapping-connector">
          <svg class="mapping-lines">
            <line
              v-for="mapping in mappings"
              :key="mapping.id"
              :x1="50"
              :y1="getFieldY(mapping.sourcePath, 'source')"
              :x2="250"
              :y2="getFieldY(mapping.targetPath, 'target')"
              stroke="#3b82f6"
              stroke-width="2"
              marker-end="url(#arrowhead)"
            />
            <defs>
              <marker id="arrowhead" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
                <polygon points="0 0, 10 3.5, 0 7" fill="#3b82f6" />
              </marker>
            </defs>
          </svg>
        </div>

        <div class="fields-column">
          <h4>目标版本字段 (v{{ targetVersion }})</h4>
          <div class="fields-list">
            <div
              v-for="field in targetFields"
              :key="field.path"
              class="field-item"
              :class="{ 'dragging': dragSource === field.path, 'mapped': isFieldMapped(field.path, 'target') }"
              draggable="true"
              @dragstart="onDragStart($event, field.path, 'target')"
              @dragover.prevent
              @drop="onDrop($event, field.path, 'target')"
            >
              <span class="field-name">{{ field.path }}</span>
              <span class="field-type">{{ field.type }}</span>
              <span v-if="field.required" class="field-required">*</span>
            </div>
            <div v-if="targetFields.length === 0" class="empty-state">
              暂无字段
            </div>
          </div>
        </div>
      </div>

      <div class="rules-panel">
        <h4>迁移规则列表</h4>
        <div v-if="rules.length === 0" class="empty-state">
          暂无规则，点击上方按钮添加规则或拖拽字段建立映射
        </div>
        <div
          v-for="(rule, index) in rules"
          :key="index"
          class="rule-card"
          :class="{ 'has-error': getRuleError(index) }"
        >
          <div class="rule-card-header">
            <span class="rule-type-badge" :class="rule.type">{{ RULE_TYPE_LABELS[rule.type] }}</span>
            <button class="btn btn-danger btn-sm" @click="removeRule(index)">×</button>
          </div>
          <div class="rule-card-body">
            <template v-if="rule.type === 'rename'">
              <div class="form-group">
                <label>源字段路径</label>
                <input v-model="rule.source_path" placeholder="e.g. user.name" />
              </div>
              <div class="form-group">
                <label>目标字段路径</label>
                <input v-model="rule.target_path" placeholder="e.g. user.full_name" />
              </div>
            </template>

            <template v-else-if="rule.type === 'delete'">
              <div class="form-group">
                <label>要删除的字段路径</label>
                <input v-model="rule.source_path" placeholder="e.g. old_field" />
              </div>
            </template>

            <template v-else-if="rule.type === 'add'">
              <div class="form-group">
                <label>新增字段路径</label>
                <input v-model="rule.target_path" placeholder="e.g. new_field" />
              </div>
              <div class="form-group">
                <label>默认值</label>
                <input v-model="defaultValues[index]" @input="updateDefaultValue(index)" placeholder="e.g. default_value" />
              </div>
            </template>

            <template v-else-if="rule.type === 'convert'">
              <div class="form-group">
                <label>字段路径</label>
                <input v-model="rule.source_path" placeholder="e.g. amount" />
              </div>
              <div class="form-group">
                <label>转换为类型</label>
                <select v-model="rule.target_type">
                  <option v-for="t in TARGET_TYPES" :key="t.value" :value="t.value">{{ t.label }}</option>
                </select>
              </div>
            </template>

            <template v-else-if="rule.type === 'map_path'">
              <div class="form-group">
                <label>源字段路径</label>
                <input v-model="rule.source_path" placeholder="e.g. data.info" />
              </div>
              <div class="form-group">
                <label>目标字段路径</label>
                <input v-model="rule.target_path" placeholder="e.g. info" />
              </div>
            </template>
          </div>
          <div v-if="getRuleError(index)" class="rule-error">
            {{ getRuleError(index) }}
          </div>
        </div>
      </div>
    </div>

    <div v-if="validationErrors.length > 0" class="validation-errors">
      <h4>规则校验错误</h4>
      <ul>
        <li v-for="(err, idx) in validationErrors" :key="idx">
          规则{{ err.rule_index + 1 }} - {{ err.field }}: {{ err.message }}
        </li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { PropType } from 'vue'
import type { MigrationRule, MigrationRuleValidationError, SchemaField, MigrationRuleType } from '../types/migration'
import { RULE_TYPE_LABELS, TARGET_TYPES } from '../types/migration'

const props = defineProps({
  sourceVersion: { type: Number, required: true },
  targetVersion: { type: Number, required: true },
  sourceSchemaDef: { type: String, default: '' },
  targetSchemaDef: { type: String, default: '' },
  modelValue: { type: Array as PropType<MigrationRule[]>, required: true }
})

const emit = defineEmits<{
  'update:modelValue': [rules: MigrationRule[]]
  'validation-errors': [errors: MigrationRuleValidationError[]]
}>()

const { parseSchemaFields, generateId } = useMigration()

const sourceFields = computed<SchemaField[]>(() => parseSchemaFields(props.sourceSchemaDef))
const targetFields = computed<SchemaField[]>(() => parseSchemaFields(props.targetSchemaDef))

const rules = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const defaultValues = ref<Record<number, string>>({})
const validationErrors = ref<MigrationRuleValidationError[]>([])
const dragSource = ref<{ path: string; side: string } | null>(null)
const mappings = ref<{ id: string; sourcePath: string; targetPath: string; ruleType: MigrationRuleType }[]>([])

const addRule = (type: MigrationRuleType) => {
  const newRule: MigrationRule = { type }
  if (type === 'convert') {
    newRule.target_type = 'string'
  }
  rules.value = [...rules.value, newRule]
}

const removeRule = (index: number) => {
  rules.value = rules.value.filter((_, i) => i !== index)
  const newDefaults: Record<number, string> = {}
  Object.entries(defaultValues.value).forEach(([idx, val]) => {
    const numIdx = parseInt(idx)
    if (numIdx < index) {
      newDefaults[numIdx] = val
    } else if (numIdx > index) {
      newDefaults[numIdx - 1] = val
    }
  })
  defaultValues.value = newDefaults
  updateMappings()
}

const updateDefaultValue = (index: number) => {
  const val = defaultValues.value[index]
  try {
    rules.value[index].default_value = JSON.parse(val)
  } catch {
    rules.value[index].default_value = val
  }
}

const onDragStart = (event: DragEvent, path: string, side: string) => {
  dragSource.value = { path, side }
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'copy'
    event.dataTransfer.setData('text/plain', path)
  }
}

const onDrop = (event: DragEvent, path: string, side: string) => {
  if (!dragSource.value) return
  if (dragSource.value.side === side) return

  if (dragSource.value.side === 'source' && side === 'target') {
    const existingMapping = mappings.value.find(
      m => m.sourcePath === dragSource.value!.path && m.targetPath === path
    )
    if (!existingMapping) {
      const newRule: MigrationRule = {
        type: 'map_path',
        source_path: dragSource.value.path,
        target_path: path
      }
      rules.value = [...rules.value, newRule]
      updateMappings()
    }
  } else if (dragSource.value.side === 'target' && side === 'source') {
    const existingMapping = mappings.value.find(
      m => m.sourcePath === path && m.targetPath === dragSource.value!.path
    )
    if (!existingMapping) {
      const newRule: MigrationRule = {
        type: 'map_path',
        source_path: path,
        target_path: dragSource.value.path
      }
      rules.value = [...rules.value, newRule]
      updateMappings()
    }
  }

  dragSource.value = null
}

const updateMappings = () => {
  mappings.value = rules.value
    .filter(r => (r.type === 'map_path' || r.type === 'rename') && r.source_path && r.target_path)
    .map(r => ({
      id: generateId(),
      sourcePath: r.source_path!,
      targetPath: r.target_path!,
      ruleType: r.type
    }))
}

const isFieldMapped = (path: string, side: 'source' | 'target') => {
  if (side === 'source') {
    return mappings.value.some(m => m.sourcePath === path)
  }
  return mappings.value.some(m => m.targetPath === path)
}

const getFieldY = (path: string, side: 'source' | 'target') => {
  const fields = side === 'source' ? sourceFields.value : targetFields.value
  const index = fields.findIndex(f => f.path === path)
  return index >= 0 ? index * 44 + 52 : 52
}

const getRuleError = (index: number) => {
  const err = validationErrors.value.find(e => e.rule_index === index)
  return err ? err.message : ''
}

watch(() => props.sourceSchemaDef, updateMappings, { immediate: true })
watch(() => props.targetSchemaDef, updateMappings, { immediate: true })
watch(() => rules.value, updateMappings, { deep: true })
</script>

<style scoped>
.rule-editor {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.rule-editor-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-bottom: 12px;
  border-bottom: 1px solid #e5e7eb;
}

.rule-editor-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.rule-editor-actions {
  display: flex;
  gap: 8px;
}

.rule-editor-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.fields-panel {
  display: flex;
  gap: 16px;
  background: #f9fafb;
  padding: 16px;
  border-radius: 8px;
}

.fields-column {
  flex: 1;
  min-width: 0;
}

.fields-column h4 {
  margin: 0 0 12px 0;
  font-size: 14px;
  font-weight: 600;
  color: #374151;
}

.fields-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 200px;
  overflow-y: auto;
}

.field-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: white;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  cursor: grab;
  transition: all 0.2s;
}

.field-item:hover {
  border-color: #3b82f6;
  background: #eff6ff;
}

.field-item.dragging {
  opacity: 0.5;
}

.field-item.mapped {
  border-color: #10b981;
  background: #f0fdf4;
}

.field-name {
  flex: 1;
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 12px;
  color: #1f2937;
}

.field-type {
  font-size: 11px;
  color: #6b7280;
  background: #f3f4f6;
  padding: 2px 6px;
  border-radius: 4px;
}

.field-required {
  color: #ef4444;
  font-size: 14px;
}

.mapping-connector {
  width: 300px;
  position: relative;
}

.mapping-lines {
  width: 100%;
  height: 100%;
  min-height: 200px;
}

.empty-state {
  padding: 24px;
  text-align: center;
  color: #9ca3af;
  font-size: 13px;
}

.rules-panel {
  background: #f9fafb;
  padding: 16px;
  border-radius: 8px;
}

.rules-panel h4 {
  margin: 0 0 12px 0;
  font-size: 14px;
  font-weight: 600;
  color: #374151;
}

.rule-card {
  background: white;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  margin-bottom: 12px;
  overflow: hidden;
}

.rule-card.has-error {
  border-color: #ef4444;
}

.rule-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: #f9fafb;
  border-bottom: 1px solid #e5e7eb;
}

.rule-type-badge {
  font-size: 12px;
  font-weight: 600;
  padding: 4px 10px;
  border-radius: 12px;
}

.rule-type-badge.rename { background: #dbeafe; color: #1d4ed8; }
.rule-type-badge.delete { background: #fee2e2; color: #991b1b; }
.rule-type-badge.add { background: #d1fae5; color: #065f46; }
.rule-type-badge.convert { background: #fef3c7; color: #92400e; }
.rule-type-badge.map_path { background: #ede9fe; color: #5b21b6; }

.rule-card-body {
  padding: 12px;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.rule-card-body .form-group:first-child {
  grid-column: 1 / -1;
}

.rule-card-body .form-group {
  margin-bottom: 0;
}

.rule-error {
  padding: 8px 12px;
  background: #fef2f2;
  color: #991b1b;
  font-size: 12px;
  border-top: 1px solid #fee2e2;
}

.validation-errors {
  background: #fef2f2;
  border: 1px solid #fecaca;
  border-radius: 8px;
  padding: 12px 16px;
}

.validation-errors h4 {
  margin: 0 0 8px 0;
  color: #991b1b;
  font-size: 14px;
}

.validation-errors ul {
  margin: 0;
  padding-left: 20px;
  color: #991b1b;
  font-size: 13px;
}

.btn-danger {
  background: #ef4444;
  color: white;
  border: none;
  padding: 2px 8px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 16px;
  line-height: 1;
}

.btn-danger:hover {
  background: #dc2626;
}
</style>
