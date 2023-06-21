package datastax_astra

import "time"

type astraRoleEntry struct {
	RoleName string        `json:"role_name"`
	RoleId   string        `json:"role_id"`
	OrgId    string        `json:"org_id"`
	TTL      time.Duration `json:"ttl"`
	MaxTTL   time.Duration `json:"max_ttl"`
}

func (r *astraRoleEntry) ToResponseData() map[string]interface{} {
	return map[string]interface{}{
		"role_name": r.RoleName,
		"role_id":   r.RoleId,
		"org_id":    r.OrgId,
		"ttl":       r.TTL.String(),
		"max_ttl":   r.MaxTTL.String(),
	}
}
