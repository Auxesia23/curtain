package authz

import (
	"fmt"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

const casbinModel = `[request_definition]
r = sub, dom, act

[policy_definition]
p = sub, dom, act, eft

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p_eft == allow)) && !some(where (p_eft == deny))

[matchers]
m = (p.dom == "*" || r.dom == p.dom) && r.act == p.act && (r.sub == p.sub || g(r.sub, p.sub, r.dom))
`

// NewEnforcer loads persisted policies and seeds the built-in role templates.
func NewEnforcer(db *gorm.DB) (*casbin.SyncedEnforcer, error) {
	policyAdapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, fmt.Errorf("create Casbin SQLite adapter: %w", err)
	}
	model, err := model.NewModelFromString(casbinModel)
	if err != nil {
		return nil, fmt.Errorf("parse Casbin model: %w", err)
	}
	enforcer, err := casbin.NewSyncedEnforcer(model, policyAdapter)
	if err != nil {
		return nil, fmt.Errorf("create Casbin enforcer: %w", err)
	}
	enforcer.EnableAutoSave(true)

	for _, role := range Roles {
		for _, capability := range role.Capabilities {
			if _, err := enforcer.AddPolicy(RoleSubject(string(role.Role)), "*", string(capability), EffectAllow); err != nil {
				return nil, fmt.Errorf("seed Casbin role policy: %w", err)
			}
		}
	}
	return enforcer, nil
}

// InTransaction applies database and Casbin changes in one SQLite transaction.
func InTransaction(enforcer *casbin.SyncedEnforcer, fn func(*gorm.DB, casbin.IEnforcer) error) error {
	adapter, ok := enforcer.GetAdapter().(*gormadapter.Adapter)
	if !ok {
		return fmt.Errorf("Casbin enforcer is not using the GORM adapter")
	}
	return adapter.Transaction(enforcer, func(txEnforcer casbin.IEnforcer) error {
		txAdapter, ok := txEnforcer.GetAdapter().(*gormadapter.Adapter)
		if !ok {
			return fmt.Errorf("Casbin transaction is not using the GORM adapter")
		}
		return fn(txAdapter.GetDb().Session(&gorm.Session{NewDB: true}), txEnforcer)
	})
}

// Effective returns a member's allowed capabilities in catalog order.
func Effective(enforcer casbin.IEnforcer, projectID, userID uint) ([]string, error) {
	capabilities := make([]string, 0, len(Catalog))
	for _, info := range Catalog {
		allowed, err := enforcer.Enforce(UserSubject(userID), ProjectDomain(projectID), string(info.Capability))
		if err != nil {
			return nil, fmt.Errorf("evaluate capability %q: %w", info.Capability, err)
		}
		if allowed {
			capabilities = append(capabilities, string(info.Capability))
		}
	}
	return capabilities, nil
}

func UserSubject(userID uint) string      { return fmt.Sprintf("user:%d", userID) }
func RoleSubject(role string) string      { return "role:" + role }
func ProjectDomain(projectID uint) string { return fmt.Sprintf("project:%d", projectID) }

func IsValidEffect(effect string) bool { return effect == EffectAllow || effect == EffectDeny }

func RemoveMemberPolicies(enforcer casbin.IEnforcer, projectID, userID uint, role string) error {
	if _, err := enforcer.RemoveGroupingPolicy(UserSubject(userID), RoleSubject(role), ProjectDomain(projectID)); err != nil {
		return err
	}
	_, err := enforcer.RemoveFilteredPolicy(0, UserSubject(userID), ProjectDomain(projectID))
	return err
}

func RemoveProjectPolicies(enforcer casbin.IEnforcer, projectID uint) error {
	domain := ProjectDomain(projectID)
	if _, err := enforcer.RemoveFilteredPolicy(1, domain); err != nil {
		return err
	}
	_, err := enforcer.RemoveFilteredGroupingPolicy(2, domain)
	return err
}
