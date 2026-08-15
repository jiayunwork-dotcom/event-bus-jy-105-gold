export type MigrationStatus = 
  | 'draft'
  | 'running'
  | 'completed'
  | 'failed'
  | 'cancelled'
  | 'rollback_running'
  | 'rollbacked'
  | 'rollback_failed'

export type MigrationRuleType = 'rename' | 'delete' | 'add' | 'convert' | 'map_path'

export interface MigrationRule {
  type: MigrationRuleType
  source_path?: string
  target_path?: string
  default_value?: any
  source_type?: string
  target_type?: string
}

export interface SchemaMigration {
  id: string
  tenant_id: string
  event_type: string
  source_version: number
  target_version: number
  status: MigrationStatus
  migration_rules: string
  total_events: number
  processed_events: number
  failed_events: number
  error_message?: string
  started_at?: string
  completed_at?: string
  rollback_deadline?: string
  created_at: string
  updated_at: string
}

export interface MigrationPreviewResult {
  event_id: string
  original_payload: Record<string, any>
  converted_payload: Record<string, any>
  success: boolean
  error_message?: string
}

export interface MigrationProgress {
  migration_id: string
  status: string
  total_events: number
  processed_events: number
  failed_events: number
  progress_percent: number
  estimated_remaining_seconds?: number
  error_message?: string
}

export interface MigrationRuleValidationError {
  rule_index: number
  field: string
  message: string
}

export interface SchemaField {
  path: string
  name: string
  type: string
  required: boolean
}

export interface FieldMapping {
  sourcePath: string
  targetPath: string
  ruleType: MigrationRuleType
  id: string
}

export const MIGRATION_STATUS_LABELS: Record<MigrationStatus, string> = {
  draft: '草稿',
  running: '执行中',
  completed: '已完成',
  failed: '失败',
  cancelled: '已取消',
  rollback_running: '回滚中',
  rollbacked: '已回滚',
  rollback_failed: '回滚失败'
}

export const MIGRATION_STATUS_COLORS: Record<MigrationStatus, string> = {
  draft: '#6b7280',
  running: '#3b82f6',
  completed: '#10b981',
  failed: '#ef4444',
  cancelled: '#f59e0b',
  rollback_running: '#8b5cf6',
  rollbacked: '#06b6d4',
  rollback_failed: '#ef4444'
}

export const RULE_TYPE_LABELS: Record<MigrationRuleType, string> = {
  rename: '重命名字段',
  delete: '删除字段',
  add: '新增字段',
  convert: '类型转换',
  map_path: '路径映射'
}

export const TARGET_TYPES = [
  { value: 'string', label: '字符串 (string)' },
  { value: 'number', label: '数字 (number)' },
  { value: 'integer', label: '整数 (integer)' },
  { value: 'boolean', label: '布尔值 (boolean)' },
  { value: 'array', label: '数组 (array)' },
  { value: 'object', label: '对象 (object)' }
]

export interface AffectedSubscription {
  id: string
  name: string
  event_type: string
  filter_expression: string
  matched_fields: string[]
}

export interface AffectedDAGNode {
  subscription_id: string
  subscription_name: string
  dag_id: string
  node_id: string
  node_name: string
  node_type: string
  config: Record<string, any>
  matched_fields: string[]
}

export interface MigrationImpactReport {
  migration_id: string
  affected_fields: string[]
  affected_subscriptions: AffectedSubscription[]
  affected_dag_nodes: AffectedDAGNode[]
  suggestions: string[]
  has_impact: boolean
}
