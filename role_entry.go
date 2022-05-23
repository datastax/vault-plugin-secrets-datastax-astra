package datastax_astra

type roleEntry struct {
	RoleId string `json:"role_id"`
	OrgId  string `json:"org_id"`
	Name   string `json:"role_name"`
}

func (r *roleEntry) ToResponseData() map[string]interface{} {
	return map[string]interface{}{
		"role_id":   r.RoleId,
		"org_id":    r.OrgId,
		"role_name": r.Name,
	}
}
