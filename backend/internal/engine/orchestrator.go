package engine

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/eventbus/server/internal/model"
	"github.com/eventbus/server/internal/repository"
)

type Orchestrator struct {
	traceRepo *repository.TraceRepo
}

func NewOrchestrator(traceRepo *repository.TraceRepo) *Orchestrator {
	return &Orchestrator{traceRepo: traceRepo}
}

func (o *Orchestrator) ValidateDAG(nodes []model.DAGNode, edges []model.DAGEdge) error {
	if len(nodes) == 0 {
		return fmt.Errorf("DAG must have at least one node")
	}

	nodeMap := make(map[string]*model.DAGNode)
	for i := range nodes {
		nodeMap[nodes[i].ID] = &nodes[i]
	}

	for _, edge := range edges {
		if _, ok := nodeMap[edge.From]; !ok {
			return fmt.Errorf("edge references unknown source node: %s", edge.From)
		}
		if _, ok := nodeMap[edge.To]; !ok {
			return fmt.Errorf("edge references unknown target node: %s", edge.To)
		}
	}

	if hasCycle(nodes, edges) {
		return fmt.Errorf("DAG contains a cycle, which is not allowed")
	}

	return nil
}

func hasCycle(nodes []model.DAGNode, edges []model.DAGEdge) bool {
	adj := make(map[string][]string)
	inDegree := make(map[string]int)

	for _, n := range nodes {
		inDegree[n.ID] = 0
	}

	for _, e := range edges {
		adj[e.From] = append(adj[e.From], e.To)
		inDegree[e.To]++
	}

	queue := []string{}
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	count := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		count++

		for _, next := range adj[node] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	return count != len(nodes)
}

type ExecutionContext struct {
	EventID        string
	SubscriptionID string
	TenantID       string
	Payload        map[string]interface{}
	Nodes          []model.DAGNode
	Edges          []model.DAGEdge
	Results        map[string]*NodeResult
}

type NodeResult struct {
	Status       string
	Output       map[string]interface{}
	ErrorMessage string
	DurationMs   int
}

func (o *Orchestrator) Execute(ctx *ExecutionContext) error {
	nodeMap := make(map[string]*model.DAGNode)
	for i := range ctx.Nodes {
		nodeMap[ctx.Nodes[i].ID] = &ctx.Nodes[i]
	}

	inDegree := make(map[string]int)
	adj := make(map[string][]model.DAGEdge)
	for _, n := range ctx.Nodes {
		inDegree[n.ID] = 0
	}
	for _, e := range ctx.Edges {
		adj[e.From] = append(adj[e.From], e)
		inDegree[e.To]++
	}

	var roots []string
	for id, deg := range inDegree {
		if deg == 0 {
			roots = append(roots, id)
		}
	}

	ctx.Results = make(map[string]*NodeResult)
	return o.executeNodes(ctx, roots, adj, nodeMap)
}

func (o *Orchestrator) executeNodes(ctx *ExecutionContext, nodeIDs []string, adj map[string][]model.DAGEdge, nodeMap map[string]*model.DAGNode) error {
	for _, nodeID := range nodeIDs {
		node := nodeMap[nodeID]

		input := ctx.Payload
		if len(ctx.Results) > 0 {
			for _, e := range ctx.Edges {
				if e.To == nodeID {
					if prev, ok := ctx.Results[e.From]; ok && prev.Status == "success" {
						if node.Type == "serial" || node.Type == "transform" {
							input = prev.Output
						}
					}
				}
			}
		}

		start := time.Now()
		result := o.executeNode(ctx, node, input)
		result.DurationMs = int(time.Since(start).Milliseconds())

		ctx.Results[nodeID] = result

		o.traceRepo.Create(&model.DeliveryTrace{
			EventID:        ctx.EventID,
			SubscriptionID: ctx.SubscriptionID,
			TenantID:       ctx.TenantID,
			NodeID:         nodeID,
			NodeType:       node.Type,
			NodeName:       node.Name,
			Status:         result.Status,
			InputPayload:   marshalJSON(input),
			OutputPayload:  marshalJSON(result.Output),
			ErrorMessage:   result.ErrorMessage,
			DurationMs:     result.DurationMs,
		})

		if node.Type == "serial" && result.Status == "failed" {
			return fmt.Errorf("serial chain failed at node %s: %s", node.Name, result.ErrorMessage)
		}

		if result.Status == "success" || node.Type == "fanout" {
			var nextNodes []string
			for _, e := range adj[nodeID] {
				if e.Condition != "" {
					if evaluateCondition(e.Condition, result.Output) {
						nextNodes = append(nextNodes, e.To)
					}
				} else {
					nextNodes = append(nextNodes, e.To)
				}
			}
			if len(nextNodes) > 0 {
				o.executeNodes(ctx, nextNodes, adj, nodeMap)
			}
		}
	}
	return nil
}

func (o *Orchestrator) executeNode(ctx *ExecutionContext, node *model.DAGNode, input map[string]interface{}) *NodeResult {
	switch node.Type {
	case "fanout":
		return &NodeResult{Status: "success", Output: input}
	case "serial":
		return &NodeResult{Status: "success", Output: input}
	case "condition":
		return o.executeCondition(node, input)
	case "transform":
		return o.executeTransform(node, input)
	case "consumer":
		return o.executeConsumer(node, input)
	default:
		return &NodeResult{Status: "success", Output: input}
	}
}

func (o *Orchestrator) executeCondition(node *model.DAGNode, input map[string]interface{}) *NodeResult {
	conditionExpr, _ := node.Config["expression"].(string)
	if conditionExpr == "" {
		return &NodeResult{Status: "success", Output: input}
	}
	return &NodeResult{Status: "success", Output: input}
}

func (o *Orchestrator) executeTransform(node *model.DAGNode, input map[string]interface{}) *NodeResult {
	mappings, _ := node.Config["mappings"].(map[string]interface{})
	if len(mappings) == 0 {
		return &NodeResult{Status: "success", Output: input}
	}

	output := make(map[string]interface{})
	for k, v := range input {
		output[k] = v
	}
	for targetField, sourceField := range mappings {
		if sf, ok := sourceField.(string); ok {
			if val, exists := getNestedField(input, sf); exists {
				setNestedField(output, targetField, val)
			}
		}
	}
	return &NodeResult{Status: "success", Output: output}
}

func (o *Orchestrator) executeConsumer(node *model.DAGNode, input map[string]interface{}) *NodeResult {
	return &NodeResult{Status: "success", Output: input}
}

func getNestedField(data map[string]interface{}, field string) (interface{}, bool) {
	parts := splitPath(field)
	var current interface{} = data
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func setNestedField(data map[string]interface{}, field string, value interface{}) {
	parts := splitPath(field)
	current := data
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return
		}
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			newMap := make(map[string]interface{})
			current[part] = newMap
			current = newMap
		}
	}
}

func splitPath(field string) []string {
	result := []string{}
	current := ""
	for _, c := range field {
		if c == '.' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func evaluateCondition(condition string, data map[string]interface{}) bool {
	return true
}

func marshalJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
