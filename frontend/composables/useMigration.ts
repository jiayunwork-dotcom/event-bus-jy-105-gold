import { ref } from 'vue'
import type { 
  MigrationRule, 
  SchemaMigration, 
  MigrationPreviewResult, 
  MigrationProgress,
  MigrationRuleValidationError,
  MigrationImpactReport
} from '../types/migration'

export function useMigration() {
  const api = useApiStore()

  const validateRules = async (rules: MigrationRule[]) => {
    return await api.post('/migrations/validate-rules', { migration_rules: rules })
  }

  const previewMigration = async (
    eventType: string,
    sourceVersion: number,
    targetVersion: number,
    rules: MigrationRule[]
  ): Promise<MigrationPreviewResult[]> => {
    return await api.post('/migrations/preview', {
      event_type: eventType,
      source_version: sourceVersion,
      target_version: targetVersion,
      migration_rules: rules
    })
  }

  const startMigration = async (
    eventType: string,
    sourceVersion: number,
    targetVersion: number,
    rules: MigrationRule[]
  ): Promise<SchemaMigration> => {
    return await api.post('/migrations/start', {
      event_type: eventType,
      source_version: sourceVersion,
      target_version: targetVersion,
      migration_rules: rules
    })
  }

  const getMigrationProgress = async (migrationId: string): Promise<MigrationProgress> => {
    return await api.get(`/migrations/${migrationId}/progress`)
  }

  const cancelMigration = async (migrationId: string) => {
    return await api.post(`/migrations/${migrationId}/cancel`, {})
  }

  const rollbackMigration = async (migrationId: string) => {
    return await api.post(`/migrations/${migrationId}/rollback`, {})
  }

  const listMigrations = async (eventType: string): Promise<SchemaMigration[]> => {
    return await api.get(`/migrations?event_type=${encodeURIComponent(eventType)}`)
  }

  const getMigration = async (migrationId: string): Promise<SchemaMigration> => {
    return await api.get(`/migrations/${migrationId}`)
  }

  const analyzeImpact = async (migrationId: string): Promise<MigrationImpactReport> => {
    return await api.get(`/migrations/${migrationId}/impact`)
  }

  const parseSchemaFields = (schemaDef: string) => {
    try {
      const schema = JSON.parse(schemaDef)
      const fields: { path: string; name: string; type: string; required: boolean }[] = []
      const requiredSet = new Set(schema.required || [])
      
      const parseProperties = (props: Record<string, any>, parentPath = '') => {
        for (const [name, prop] of Object.entries(props)) {
          const path = parentPath ? `${parentPath}.${name}` : name
          const type = prop.type || 'object'
          
          fields.push({
            path,
            name,
            type: Array.isArray(type) ? type.join('/') : type,
            required: requiredSet.has(name)
          })
          
          if (prop.properties && type === 'object') {
            parseProperties(prop.properties, path)
          }
        }
      }
      
      if (schema.properties) {
        parseProperties(schema.properties)
      }
      
      return fields
    } catch {
      return []
    }
  }

  const generateId = () => Math.random().toString(36).substring(2, 11)

  const formatTimeRemaining = (seconds: number) => {
    if (seconds < 60) return `${Math.round(seconds)}秒`
    if (seconds < 3600) return `${Math.round(seconds / 60)}分钟`
    return `${Math.round(seconds / 3600)}小时${Math.round((seconds % 3600) / 60)}分钟`
  }

  return {
    validateRules,
    previewMigration,
    startMigration,
    getMigrationProgress,
    cancelMigration,
    rollbackMigration,
    listMigrations,
    getMigration,
    analyzeImpact,
    parseSchemaFields,
    generateId,
    formatTimeRemaining
  }
}
