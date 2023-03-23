package datastax_astra

// astraConfig includes the minimum configuration
// required to instantiate a new astra client.
type astraConfig struct {
    AstraToken  string     `json:"astra_token"`
    URL         string     `json:"url"`
    OrgId       string     `json:"org_id"`
    LogicalName string     `json:"logical_name"`
    CallerMode  CallerMode `json:"caller_mode"`
}

func (c *astraConfig) ToResponseData() map[string]interface{} {
    return map[string]interface{}{
        "astra_token":  c.AstraToken,
        "url":          c.URL,
        "org_id":       c.OrgId,
        "logical_name": c.LogicalName,
        "caller_mode":  c.CallerMode.String(),
    }
}
