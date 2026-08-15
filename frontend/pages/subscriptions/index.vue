<template>
  <div>
    <div class="page-header">
      <h1>订阅与编排</h1>
      <button class="btn btn-primary" @click="showCreateModal = true">+ 创建订阅</button>
    </div>

    <div class="card">
      <table>
        <thead>
          <tr>
            <th>订阅名称</th>
            <th>事件类型</th>
            <th>投递模式</th>
            <th>过滤表达式</th>
            <th>消费端URL</th>
            <th>速率限制</th>
            <th>状态</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="sub in subscriptions" :key="sub.id">
            <td style="font-weight: 600;">{{ sub.name }}</td>
            <td><code style="background: #f3f4f6; padding: 2px 8px; border-radius: 4px; font-size: 12px;">{{ sub.event_type }}</code></td>
            <td>
              <span :class="['badge', sub.delivery_mode === 'exactly_once' ? 'badge-delivered' : 'badge-active']">
                {{ sub.delivery_mode === 'exactly_once' ? 'Exactly-Once' : 'At-Least-Once' }}
              </span>
            </td>
            <td style="max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
              {{ sub.filter_expression || '-' }}
            </td>
            <td style="font-size: 12px; max-width: 200px; overflow: hidden; text-overflow: ellipsis;">{{ sub.consumer_url }}</td>
            <td>{{ sub.consumer_rate_limit }}/s</td>
            <td><span :class="['badge', `badge-${sub.status}`]">{{ sub.status }}</span></td>
            <td>
              <button class="btn btn-outline btn-sm" @click="editSubscription(sub)">编辑</button>
              <button class="btn btn-outline btn-sm" style="margin-left: 8px;" @click="openDAGEditor(sub)">编排</button>
              <button class="btn btn-danger btn-sm" style="margin-left: 8px;" @click="deleteSubscription(sub.id)">删除</button>
            </td>
          </tr>
          <tr v-if="subscriptions.length === 0">
            <td colspan="8" style="text-align: center; color: #9ca3af; padding: 40px;">暂无订阅数据</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="showCreateModal" class="modal-overlay" @click.self="closeCreateModal">
      <div class="modal" style="max-width: 720px;">
        <h2>{{ editingSub ? '编辑订阅' : '创建订阅' }}</h2>
        <div class="grid-2">
          <div class="form-group">
            <label>订阅名称</label>
            <input v-model="subForm.name" placeholder="输入订阅名称" />
          </div>
          <div class="form-group">
            <label>事件类型</label>
            <input v-model="subForm.event_type" placeholder="e.g. order.created" />
          </div>
        </div>
        <div class="form-group">
          <label>消费端URL</label>
          <input v-model="subForm.consumer_url" placeholder="https://consumer.example.com/webhook" />
        </div>
        <div class="grid-2">
          <div class="form-group">
            <label>投递模式</label>
            <select v-model="subForm.delivery_mode">
              <option value="at_least_once">At-Least-Once</option>
              <option value="exactly_once">Exactly-Once</option>
            </select>
          </div>
          <div class="form-group">
            <label>最大重试次数</label>
            <input v-model.number="subForm.max_retries" type="number" min="0" max="5" />
          </div>
        </div>
        <div v-if="subForm.delivery_mode === 'exactly_once'" class="grid-2">
          <div class="form-group">
            <label>幂等Key路径</label>
            <input v-model="subForm.idempotent_key_path" placeholder="e.g. order_id" />
          </div>
          <div class="form-group">
            <label>去重窗口(秒)</label>
            <input v-model.number="subForm.idempotent_window_seconds" type="number" />
          </div>
        </div>
        <div class="form-group">
          <label>过滤表达式 (CEL语法)</label>
          <textarea v-model="subForm.filter_expression" style="min-height: 60px; font-family: 'Monaco', monospace; font-size: 13px;"
            placeholder='e.g. payload.status == "active" && payload.amount > 100'></textarea>
        </div>
        <div class="grid-2">
          <div class="form-group">
            <label>消费速率上限(req/s)</label>
            <input v-model.number="subForm.consumer_rate_limit" type="number" />
          </div>
          <div class="form-group">
            <label>桶容量(突发)</label>
            <input v-model.number="subForm.consumer_burst" type="number" />
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn btn-outline" @click="closeCreateModal">取消</button>
          <button class="btn btn-primary" @click="submitSub">{{ editingSub ? '保存' : '创建' }}</button>
        </div>
      </div>
    </div>

    <div v-if="showDAGModal" class="modal-overlay" @click.self="showDAGModal = false">
      <div class="modal" style="max-width: 1100px; width: 95%;">
        <h2>编排DAG编辑器 - {{ dagSubscription?.name }}</h2>
        <div style="display: flex; gap: 12px; margin-bottom: 16px;">
          <button class="btn btn-outline btn-sm" @click="addNode('fanout')">+ 扇出节点</button>
          <button class="btn btn-outline btn-sm" @click="addNode('serial')">+ 串行节点</button>
          <button class="btn btn-outline btn-sm" @click="addNode('condition')">+ 条件路由</button>
          <button class="btn btn-outline btn-sm" @click="addNode('transform')">+ 转换节点</button>
          <button class="btn btn-outline btn-sm" @click="addNode('consumer')">+ 消费者节点</button>
        </div>
        <div class="dag-editor" ref="dagContainer">
          <svg width="100%" height="100%" @click="handleDAGClick">
            <defs>
              <marker id="arrowhead" markerWidth="10" markerHeight="7" refX="10" refY="3.5" orient="auto">
                <polygon points="0 0, 10 3.5, 0 7" fill="#3b82f6" />
              </marker>
            </defs>
            <g v-for="(edge, idx) in dagEdges" :key="'edge-'+idx">
              <line
                :x1="getNodeCenter(edge.from).x" :y1="getNodeCenter(edge.from).y"
                :x2="getNodeCenter(edge.to).x" :y2="getNodeCenter(edge.to).y"
                stroke="#3b82f6" stroke-width="2" marker-end="url(#arrowhead)"
              />
              <text v-if="edge.condition"
                :x="(getNodeCenter(edge.from).x + getNodeCenter(edge.to).x) / 2"
                :y="(getNodeCenter(edge.from).y + getNodeCenter(edge.to).y) / 2 - 8"
                fill="#6b7280" font-size="11" text-anchor="middle">{{ edge.condition }}</text>
            </g>
            <g v-for="node in dagNodes" :key="node.id"
               :transform="`translate(${node.position?.x || 100}, ${node.position?.y || 100})`"
               @mousedown.stop="startDrag(node, $event)"
               @click.stop="selectNode(node)">
              <rect width="180" height="60" rx="8"
                :fill="selectedNode?.id === node.id ? '#dbeafe' : '#ffffff'"
                :stroke="getNodeColor(node.type)" stroke-width="2" />
              <text x="90" y="25" text-anchor="middle" fill="#1f2937" font-size="13" font-weight="600">{{ node.name }}</text>
              <text x="90" y="45" text-anchor="middle" :fill="getNodeColor(node.type)" font-size="11">{{ node.type }}</text>
            </g>
          </svg>
        </div>

        <div v-if="selectedNode" style="margin-top: 16px; padding: 16px; background: #f9fafb; border-radius: 8px;">
          <h3 style="margin-bottom: 12px;">节点配置 - {{ selectedNode.name }}</h3>
          <div class="grid-2">
            <div class="form-group">
              <label>节点名称</label>
              <input v-model="selectedNode.name" />
            </div>
            <div class="form-group">
              <label>节点类型</label>
              <select v-model="selectedNode.type" disabled>
                <option value="fanout">扇出</option>
                <option value="serial">串行</option>
                <option value="condition">条件路由</option>
                <option value="transform">转换</option>
                <option value="consumer">消费者</option>
              </select>
            </div>
          </div>
          <div v-if="selectedNode.type === 'condition'" class="form-group">
            <label>条件表达式</label>
            <input v-model="selectedNode.config.expression" placeholder='e.g. payload.status == "success"' />
          </div>
          <div v-if="selectedNode.type === 'transform'" class="form-group">
            <label>字段映射 (JSON)</label>
            <textarea v-model="selectedNode.config.mappings_json" style="min-height: 80px; font-family: monospace;"
              placeholder='{"output_name": "payload.source_field"}'></textarea>
          </div>
          <div v-if="selectedNode.type === 'consumer'" class="form-group">
            <label>消费者URL</label>
            <input v-model="selectedNode.config.url" placeholder="https://consumer.example.com/handle" />
          </div>
          <div style="margin-top: 12px;">
            <button class="btn btn-outline btn-sm" @click="removeNode(selectedNode.id)">删除节点</button>
          </div>
        </div>

        <div style="margin-top: 16px;">
          <h4>连线</h4>
          <div class="grid-3" style="margin-top: 8px;">
            <div class="form-group">
              <label>源节点</label>
              <select v-model="newEdge.from">
                <option v-for="n in dagNodes" :key="n.id" :value="n.id">{{ n.name }}</option>
              </select>
            </div>
            <div class="form-group">
              <label>目标节点</label>
              <select v-model="newEdge.to">
                <option v-for="n in dagNodes" :key="n.id" :value="n.id">{{ n.name }}</option>
              </select>
            </div>
            <div class="form-group">
              <label>条件(可选)</label>
              <input v-model="newEdge.condition" placeholder="路由条件" />
            </div>
          </div>
          <button class="btn btn-outline btn-sm" @click="addEdge">添加连线</button>
        </div>

        <div class="modal-actions" style="margin-top: 24px;">
          <button class="btn btn-outline" @click="showDAGModal = false">取消</button>
          <button class="btn btn-primary" @click="saveDAG">保存编排</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
const api = useApiStore()
const subscriptions = ref<any[]>([])
const showCreateModal = ref(false)
const showDAGModal = ref(false)
const editingSub = ref<any>(null)
const dagSubscription = ref<any>(null)
const selectedNode = ref<any>(null)
const dagContainer = ref<HTMLElement | null>(null)

const subForm = ref({
  name: '',
  event_type: '',
  consumer_url: '',
  delivery_mode: 'at_least_once',
  filter_expression: '',
  max_retries: 5,
  idempotent_key_path: '',
  idempotent_window_seconds: 86400,
  consumer_rate_limit: 100,
  consumer_burst: 200,
})

const dagNodes = ref<any[]>([])
const dagEdges = ref<any[]>([])
const newEdge = ref({ from: '', to: '', condition: '' })

let dragNode: any = null
let dragOffset = { x: 0, y: 0 }

const fetchSubscriptions = async () => {
  if (!api.tenantId) return
  try {
    subscriptions.value = await api.get('/subscriptions')
  } catch (e) {}
}

const editSubscription = (sub: any) => {
  editingSub.value = sub
  subForm.value = {
    name: sub.name,
    event_type: sub.event_type,
    consumer_url: sub.consumer_url,
    delivery_mode: sub.delivery_mode,
    filter_expression: sub.filter_expression || '',
    max_retries: sub.max_retries,
    idempotent_key_path: sub.idempotent_key_path || '',
    idempotent_window_seconds: sub.idempotent_window_seconds,
    consumer_rate_limit: sub.consumer_rate_limit,
    consumer_burst: sub.consumer_burst,
  }
  showCreateModal.value = true
}

const closeCreateModal = () => {
  showCreateModal.value = false
  editingSub.value = null
  subForm.value = {
    name: '', event_type: '', consumer_url: '',
    delivery_mode: 'at_least_once', filter_expression: '',
    max_retries: 5, idempotent_key_path: '', idempotent_window_seconds: 86400,
    consumer_rate_limit: 100, consumer_burst: 200,
  }
}

const submitSub = async () => {
  try {
    if (editingSub.value) {
      await api.put(`/subscriptions/${editingSub.value.id}`, subForm.value)
    } else {
      await api.post('/subscriptions', subForm.value)
    }
    closeCreateModal()
    fetchSubscriptions()
  } catch (e) {}
}

const deleteSubscription = async (id: string) => {
  if (!confirm('确定删除此订阅?')) return
  try {
    await api.del(`/subscriptions/${id}`)
    fetchSubscriptions()
  } catch (e) {}
}

const openDAGEditor = async (sub: any) => {
  dagSubscription.value = sub
  selectedNode.value = null
  try {
    const dag = await api.get(`/subscriptions/${sub.id}/dag`)
    dagNodes.value = dag.nodes || []
    dagEdges.value = dag.edges || []
  } catch (e) {
    dagNodes.value = []
    dagEdges.value = []
  }
  showDAGModal.value = true
}

const addNode = (type: string) => {
  const names: Record<string, string> = {
    fanout: '扇出', serial: '串行链', condition: '条件路由', transform: '转换', consumer: '消费者'
  }
  const id = `node_${Date.now()}`
  dagNodes.value.push({
    id,
    type,
    name: `${names[type]} ${dagNodes.value.length + 1}`,
    config: {},
    position: { x: 100 + (dagNodes.value.length % 3) * 220, y: 80 + Math.floor(dagNodes.value.length / 3) * 100 }
  })
}

const removeNode = (id: string) => {
  dagNodes.value = dagNodes.value.filter((n: any) => n.id !== id)
  dagEdges.value = dagEdges.value.filter((e: any) => e.from !== id && e.to !== id)
  selectedNode.value = null
}

const addEdge = () => {
  if (newEdge.value.from && newEdge.value.to && newEdge.value.from !== newEdge.value.to) {
    dagEdges.value.push({ ...newEdge.value })
    newEdge.value = { from: '', to: '', condition: '' }
  }
}

const selectNode = (node: any) => {
  selectedNode.value = node
}

const getNodeCenter = (nodeId: string) => {
  const node = dagNodes.value.find((n: any) => n.id === nodeId)
  return { x: (node?.position?.x || 0) + 90, y: (node?.position?.y || 0) + 30 }
}

const getNodeColor = (type: string) => {
  const colors: Record<string, string> = {
    fanout: '#8b5cf6', serial: '#3b82f6', condition: '#f59e0b', transform: '#22c55e', consumer: '#ef4444'
  }
  return colors[type] || '#6b7280'
}

const startDrag = (node: any, event: MouseEvent) => {
  dragNode = node
  const svgEl = dagContainer.value?.querySelector('svg')
  if (!svgEl) return
  const rect = svgEl.getBoundingClientRect()
  dragOffset.x = event.clientX - rect.left - (node.position?.x || 0)
  dragOffset.y = event.clientY - rect.top - (node.position?.y || 0)
}

const handleDAGClick = () => {
  selectedNode.value = null
}

if (typeof window !== 'undefined') {
  window.addEventListener('mousemove', (e: MouseEvent) => {
    if (!dragNode) return
    const svgEl = dagContainer.value?.querySelector('svg')
    if (!svgEl) return
    const rect = svgEl.getBoundingClientRect()
    dragNode.position = {
      x: e.clientX - rect.left - dragOffset.x,
      y: e.clientY - rect.top - dragOffset.y,
    }
  })
  window.addEventListener('mouseup', () => {
    dragNode = null
  })
}

const saveDAG = async () => {
  if (!dagSubscription.value) return
  try {
    const nodes = dagNodes.value.map((n: any) => ({
      ...n,
      config: n.type === 'transform' && n.config?.mappings_json
        ? { mappings: JSON.parse(n.config.mappings_json) }
        : n.config
    }))
    await api.put(`/subscriptions/${dagSubscription.value.id}/dag`, {
      nodes,
      edges: dagEdges.value,
    })
    showDAGModal.value = false
  } catch (e) {
    alert('保存DAG失败: ' + (e as Error).message)
  }
}

onMounted(async () => {
  await fetchSubscriptions()
  const route = useRoute()
  if (route.query.edit && subscriptions.value.length > 0) {
    const sub = subscriptions.value.find((s: any) => s.id === route.query.edit)
    if (sub) {
      editSubscription(sub)
    }
  } else if (route.query.dag && subscriptions.value.length > 0) {
    const sub = subscriptions.value.find((s: any) => s.id === route.query.dag)
    if (sub) {
      openDAGEditor(sub)
    }
  }
})
</script>
