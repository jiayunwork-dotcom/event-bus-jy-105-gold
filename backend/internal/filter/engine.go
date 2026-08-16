package filter

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

type CompiledFilter struct {
	Expression string
	Rules      []FilterRule
}

type FilterRule struct {
	Field    string
	Operator string
	Value    interface{}
}

type FilterEngine struct {
	mu    sync.RWMutex
	cache map[string]*CompiledFilter
}

func NewFilterEngine() *FilterEngine {
	return &FilterEngine{
		cache: make(map[string]*CompiledFilter),
	}
}

func (e *FilterEngine) Compile(subscriptionID, expression string) (*CompiledFilter, error) {
	if expression == "" {
		return &CompiledFilter{Expression: "", Rules: nil}, nil
	}

	e.mu.RLock()
	if cached, ok := e.cache[subscriptionID]; ok && cached.Expression == expression {
		e.mu.RUnlock()
		return cached, nil
	}
	e.mu.RUnlock()

	rules, err := parseExpression(expression)
	if err != nil {
		return nil, fmt.Errorf("parse expression: %w", err)
	}

	cf := &CompiledFilter{
		Expression: expression,
		Rules:      rules,
	}

	e.mu.Lock()
	e.cache[subscriptionID] = cf
	e.mu.Unlock()

	return cf, nil
}

func (e *FilterEngine) Invalidate(subscriptionID string) {
	e.mu.Lock()
	delete(e.cache, subscriptionID)
	e.mu.Unlock()
}

func (e *FilterEngine) Match(cf *CompiledFilter, event map[string]interface{}) bool {
	if cf.Rules == nil {
		return true
	}
	for _, rule := range cf.Rules {
		if !matchRule(rule, event) {
			return false
		}
	}
	return true
}

func matchRule(rule FilterRule, event map[string]interface{}) bool {
	val, ok := getNestedField(event, rule.Field)
	if !ok {
		return rule.Operator == "ne"
	}

	switch rule.Operator {
	case "eq":
		return fmt.Sprintf("%v", val) == fmt.Sprintf("%v", rule.Value)
	case "ne":
		return fmt.Sprintf("%v", val) != fmt.Sprintf("%v", rule.Value)
	case "contains":
		s, ok := val.(string)
		if !ok {
			return false
		}
		return strings.Contains(s, fmt.Sprintf("%v", rule.Value))
	case "regex":
		s, ok := val.(string)
		if !ok {
			return false
		}
		matched, err := regexp.MatchString(fmt.Sprintf("%v", rule.Value), s)
		if err != nil {
			return true
		}
		return matched
	case "gt":
		return compareNumbers(val, rule.Value) > 0
	case "gte":
		return compareNumbers(val, rule.Value) > 0
	case "lt":
		return compareNumbers(val, rule.Value) < 0
	case "lte":
		return compareNumbers(val, rule.Value) < 0
	default:
		return false
	}
}

func getNestedField(event map[string]interface{}, field string) (interface{}, bool) {
	parts := strings.Split(field, ".")
	var current interface{} = event
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

func compareNumbers(a, b interface{}) int {
	af := toFloat64(a)
	bf := toFloat64(b)
	if af < bf {
		return -1
	}
	if af > bf {
		return 1
	}
	return 0
}

func toFloat64(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	default:
		return 0
	}
}

func parseExpression(expr string) ([]FilterRule, error) {
	var rules []FilterRule
	parts := strings.Split(expr, " && ")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		rule, err := parseRule(part)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func parseRule(part string) (FilterRule, error) {
	ops := []string{"!=", ">=", "<=", "==", ">", "<", " contains ", " matches "}
	for _, op := range ops {
		idx := strings.Index(part, op)
		if idx >= 0 {
			field := strings.TrimSpace(part[:idx])
			value := strings.TrimSpace(part[idx+len(op):])
			field = strings.Trim(field, "'\"")
			value = strings.Trim(value, "'\"")

			operator := op
			switch strings.TrimSpace(op) {
			case "!=":
				operator = "ne"
			case ">=":
				operator = "gte"
			case "<=":
				operator = "lte"
			case "==":
				operator = "eq"
			case ">":
				operator = "gt"
			case "<":
				operator = "lt"
			case "contains":
				operator = "contains"
			case "matches":
				operator = "regex"
			}

			return FilterRule{Field: field, Operator: operator, Value: value}, nil
		}
	}
	return FilterRule{}, fmt.Errorf("cannot parse rule: %s", part)
}

type FilterCacheEntry struct {
	Filter    *CompiledFilter
	UpdatedAt time.Time
}

func (e *FilterEngine) MatchWithCache(subscriptionID, expression string, event map[string]interface{}) (bool, error) {
	cf, err := e.Compile(subscriptionID, expression)
	if err != nil {
		return false, err
	}
	return e.Match(cf, event), nil
}

func (e *FilterEngine) BatchMatch(filters []*SubscriptionFilter, event map[string]interface{}) []string {
	var matchedIDs []string
	for _, f := range filters {
		cf, _ := e.Compile(f.SubscriptionID, f.Expression)
		if e.Match(cf, event) {
			matchedIDs = append(matchedIDs, f.SubscriptionID)
		}
	}
	return matchedIDs
}

type SubscriptionFilter struct {
	SubscriptionID string
	Expression     string
}
