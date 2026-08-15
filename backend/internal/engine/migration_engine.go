package engine

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/eventbus/server/internal/model"
)

type MigrationEngine struct{}

func NewMigrationEngine() *MigrationEngine {
	return &MigrationEngine{}
}

func (e *MigrationEngine) ValidateRules(rules []model.MigrationRule) []model.MigrationRuleValidationError {
	var errors []model.MigrationRuleValidationError

	validTypes := map[model.MigrationRuleType]bool{
		model.RuleTypeRename:  true,
		model.RuleTypeDelete:  true,
		model.RuleTypeAdd:     true,
		model.RuleTypeConvert: true,
		model.RuleTypeMapPath: true,
	}

	validTargetTypes := map[string]bool{
		"string":  true,
		"number":  true,
		"integer": true,
		"boolean": true,
		"array":   true,
		"object":  true,
	}

	for i, rule := range rules {
		if !validTypes[rule.Type] {
			errors = append(errors, model.MigrationRuleValidationError{
				RuleIndex: i,
				Field:     "type",
				Message:   fmt.Sprintf("invalid rule type: %s", rule.Type),
			})
		}

		switch rule.Type {
		case model.RuleTypeRename:
			if rule.SourcePath == "" {
				errors = append(errors, model.MigrationRuleValidationError{
					RuleIndex: i,
					Field:     "source_path",
					Message:   "source_path is required for rename rule",
				})
			}
			if rule.TargetPath == "" {
				errors = append(errors, model.MigrationRuleValidationError{
					RuleIndex: i,
					Field:     "target_path",
					Message:   "target_path is required for rename rule",
				})
			}
			if rule.SourcePath == rule.TargetPath {
				errors = append(errors, model.MigrationRuleValidationError{
					RuleIndex: i,
					Field:     "target_path",
					Message:   "target_path must be different from source_path for rename rule",
				})
			}

		case model.RuleTypeDelete:
			if rule.SourcePath == "" {
				errors = append(errors, model.MigrationRuleValidationError{
					RuleIndex: i,
					Field:     "source_path",
					Message:   "source_path is required for delete rule",
				})
			}

		case model.RuleTypeAdd:
			if rule.TargetPath == "" {
				errors = append(errors, model.MigrationRuleValidationError{
					RuleIndex: i,
					Field:     "target_path",
					Message:   "target_path is required for add rule",
				})
			}
			if rule.DefaultValue == nil {
				errors = append(errors, model.MigrationRuleValidationError{
					RuleIndex: i,
					Field:     "default_value",
					Message:   "default_value is required for add rule",
				})
			}

		case model.RuleTypeConvert:
			if rule.SourcePath == "" {
				errors = append(errors, model.MigrationRuleValidationError{
					RuleIndex: i,
					Field:     "source_path",
					Message:   "source_path is required for convert rule",
				})
			}
			if rule.TargetType == "" {
				errors = append(errors, model.MigrationRuleValidationError{
					RuleIndex: i,
					Field:     "target_type",
					Message:   "target_type is required for convert rule",
				})
			}
			if !validTargetTypes[rule.TargetType] {
				errors = append(errors, model.MigrationRuleValidationError{
					RuleIndex: i,
					Field:     "target_type",
					Message:   fmt.Sprintf("invalid target_type: %s, must be one of: string, number, integer, boolean, array, object", rule.TargetType),
				})
			}

		case model.RuleTypeMapPath:
			if rule.SourcePath == "" {
				errors = append(errors, model.MigrationRuleValidationError{
					RuleIndex: i,
					Field:     "source_path",
					Message:   "source_path is required for map_path rule",
				})
			}
			if rule.TargetPath == "" {
				errors = append(errors, model.MigrationRuleValidationError{
					RuleIndex: i,
					Field:     "target_path",
					Message:   "target_path is required for map_path rule",
				})
			}
		}
	}

	return errors
}

func (e *MigrationEngine) ApplyRules(payload map[string]interface{}, rules []model.MigrationRule) (map[string]interface{}, error) {
	result := deepCopy(payload)

	for _, rule := range rules {
		var err error
		switch rule.Type {
		case model.RuleTypeRename:
			err = e.applyRename(result, rule.SourcePath, rule.TargetPath)
		case model.RuleTypeDelete:
			err = e.applyDelete(result, rule.SourcePath)
		case model.RuleTypeAdd:
			err = e.applyAdd(result, rule.TargetPath, rule.DefaultValue)
		case model.RuleTypeConvert:
			err = e.applyConvert(result, rule.SourcePath, rule.TargetType)
		case model.RuleTypeMapPath:
			err = e.applyMapPath(result, rule.SourcePath, rule.TargetPath)
		}
		if err != nil {
			return nil, fmt.Errorf("rule %s failed: %w", rule.Type, err)
		}
	}

	return result, nil
}

func (e *MigrationEngine) applyRename(payload map[string]interface{}, sourcePath, targetPath string) error {
	value, exists := getNestedValue(payload, sourcePath)
	if !exists {
		return nil
	}

	if err := setNestedValue(payload, targetPath, value); err != nil {
		return err
	}

	return deleteNestedValue(payload, sourcePath)
}

func (e *MigrationEngine) applyDelete(payload map[string]interface{}, sourcePath string) error {
	return deleteNestedValue(payload, sourcePath)
}

func (e *MigrationEngine) applyAdd(payload map[string]interface{}, targetPath string, defaultValue interface{}) error {
	_, exists := getNestedValue(payload, targetPath)
	if exists {
		return nil
	}
	return setNestedValue(payload, targetPath, defaultValue)
}

func (e *MigrationEngine) applyConvert(payload map[string]interface{}, sourcePath, targetType string) error {
	value, exists := getNestedValue(payload, sourcePath)
	if !exists {
		return nil
	}

	converted, err := convertValue(value, targetType)
	if err != nil {
		return fmt.Errorf("convert %s to %s failed: %w", sourcePath, targetType, err)
	}

	return setNestedValue(payload, sourcePath, converted)
}

func (e *MigrationEngine) applyMapPath(payload map[string]interface{}, sourcePath, targetPath string) error {
	value, exists := getNestedValue(payload, sourcePath)
	if !exists {
		return nil
	}
	return setNestedValue(payload, targetPath, value)
}

func convertValue(value interface{}, targetType string) (interface{}, error) {
	switch targetType {
	case "string":
		return toString(value)
	case "number":
		return toFloat64(value)
	case "integer":
		return toInt64(value)
	case "boolean":
		return toBool(value)
	case "array":
		return toArray(value)
	case "object":
		return toObject(value)
	default:
		return nil, fmt.Errorf("unsupported target type: %s", targetType)
	}
}

func toString(v interface{}) (string, error) {
	switch val := v.(type) {
	case string:
		return val, nil
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64), nil
	case int:
		return strconv.Itoa(val), nil
	case int64:
		return strconv.FormatInt(val, 10), nil
	case bool:
		return strconv.FormatBool(val), nil
	case nil:
		return "", nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case string:
		return strconv.ParseFloat(val, 64)
	case bool:
		if val {
			return 1.0, nil
		}
		return 0.0, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

func toInt64(v interface{}) (int64, error) {
	switch val := v.(type) {
	case int:
		return int64(val), nil
	case int64:
		return val, nil
	case float64:
		return int64(val), nil
	case string:
		return strconv.ParseInt(val, 10, 64)
	case bool:
		if val {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", v)
	}
}

func toBool(v interface{}) (bool, error) {
	switch val := v.(type) {
	case bool:
		return val, nil
	case string:
		return strconv.ParseBool(val)
	case int, int64:
		return val != 0, nil
	case float64:
		return val != 0, nil
	default:
		return false, fmt.Errorf("cannot convert %T to bool", v)
	}
}

func toArray(v interface{}) ([]interface{}, error) {
	switch val := v.(type) {
	case []interface{}:
		return val, nil
	case string:
		var result []interface{}
		err := json.Unmarshal([]byte(val), &result)
		if err != nil {
			return []interface{}{val}, nil
		}
		return result, nil
	default:
		return []interface{}{v}, nil
	}
}

func toObject(v interface{}) (map[string]interface{}, error) {
	switch val := v.(type) {
	case map[string]interface{}:
		return val, nil
	case string:
		var result map[string]interface{}
		err := json.Unmarshal([]byte(val), &result)
		if err != nil {
			return nil, fmt.Errorf("cannot parse string as object: %w", err)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("cannot convert %T to object", v)
	}
}

func getNestedValue(payload map[string]interface{}, path string) (interface{}, bool) {
	parts := splitMigrationPath(path)
	current := interface{}(payload)

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

func setNestedValue(payload map[string]interface{}, path string, value interface{}) error {
	parts := splitMigrationPath(path)
	current := payload

	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		next, exists := current[part]
		if !exists {
			newMap := make(map[string]interface{})
			current[part] = newMap
			current = newMap
		} else {
			m, ok := next.(map[string]interface{})
			if !ok {
				return fmt.Errorf("path %s is not an object at segment %s", path, part)
			}
			current = m
		}
	}

	current[parts[len(parts)-1]] = value
	return nil
}

func deleteNestedValue(payload map[string]interface{}, path string) error {
	parts := splitMigrationPath(path)
	current := payload

	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		next, exists := current[part]
		if !exists {
			return nil
		}
		m, ok := next.(map[string]interface{})
		if !ok {
			return fmt.Errorf("path %s is not an object at segment %s", path, part)
		}
		current = m
	}

	delete(current, parts[len(parts)-1])
	return nil
}

func splitMigrationPath(path string) []string {
	return strings.Split(path, ".")
}

func deepCopy(m map[string]interface{}) map[string]interface{} {
	b, _ := json.Marshal(m)
	var result map[string]interface{}
	json.Unmarshal(b, &result)
	return result
}
