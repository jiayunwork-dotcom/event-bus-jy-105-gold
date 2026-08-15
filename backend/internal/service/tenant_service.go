package service

import (
	"encoding/json"
	"fmt"

	"github.com/eventbus/server/internal/model"
	"github.com/eventbus/server/internal/repository"
)

type TenantService struct {
	tenantRepo *repository.TenantRepo
}

func NewTenantService(tenantRepo *repository.TenantRepo) *TenantService {
	return &TenantService{tenantRepo: tenantRepo}
}

func (s *TenantService) Create(req map[string]interface{}) (*model.Tenant, error) {
	rp := model.RetentionPolicy{Type: "time", ValueDays: 30}
	if rpRaw, ok := req["retention_policy"]; ok {
		b, _ := json.Marshal(rpRaw)
		json.Unmarshal(b, &rp)
	}

	t := &model.Tenant{
		Name:             getString(req, "name"),
		Status:           "active",
		MaxPublishQPS:    getInt(req, "max_publish_qps", 1000),
		MaxSubscriptions: getInt(req, "max_subscriptions", 100),
		MaxStorageMB:     getInt(req, "max_storage_mb", 10240),
		RetentionPolicy:  rp,
	}

	if t.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	if err := s.tenantRepo.Create(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *TenantService) Get(id string) (*model.Tenant, error) {
	return s.tenantRepo.GetByID(id)
}

func (s *TenantService) List() ([]*model.Tenant, error) {
	return s.tenantRepo.List()
}

func (s *TenantService) Update(id string, req map[string]interface{}) (*model.Tenant, error) {
	t, err := s.tenantRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if name, ok := req["name"]; ok {
		t.Name = fmt.Sprintf("%v", name)
	}
	if status, ok := req["status"]; ok {
		t.Status = fmt.Sprintf("%v", status)
	}
	if v, ok := req["max_publish_qps"]; ok {
		t.MaxPublishQPS = int(toFloat64(v))
	}
	if v, ok := req["max_subscriptions"]; ok {
		t.MaxSubscriptions = int(toFloat64(v))
	}
	if v, ok := req["max_storage_mb"]; ok {
		t.MaxStorageMB = int(toFloat64(v))
	}
	if rpRaw, ok := req["retention_policy"]; ok {
		b, _ := json.Marshal(rpRaw)
		json.Unmarshal(b, &t.RetentionPolicy)
	}

	if err := s.tenantRepo.Update(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *TenantService) Disable(id string) error {
	t, err := s.tenantRepo.GetByID(id)
	if err != nil {
		return err
	}
	t.Status = "disabled"
	return s.tenantRepo.Update(t)
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func getInt(m map[string]interface{}, key string, defaultVal int) int {
	if v, ok := m[key]; ok {
		return int(toFloat64(v))
	}
	return defaultVal
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
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}
