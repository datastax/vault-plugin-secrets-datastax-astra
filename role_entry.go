package datastax_astra

import "time"

type roleEntry struct {
	RoleId string        `json:"role_id"`
	OrgId  string        `json:"org_id"`
	Name   string        `json:"role_name"`
	TTL    time.Duration `json:"ttl"`
	MaxTTL time.Duration `json:"max_ttl"`
}

func (r *roleEntry) ToResponseData() map[string]interface{} {
	return map[string]interface{}{
		"role_id":   r.RoleId,
		"org_id":    r.OrgId,
		"role_name": r.Name,
		"ttl":       r.TTL.Seconds(),
		"max_ttl":   r.MaxTTL.Seconds(),
	}
}
