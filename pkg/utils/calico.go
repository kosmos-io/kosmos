package utils

func GetCIDRs(obj map[string]interface{}) string {
	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		return ""
	}
	cidr, ok := spec["cidr"].(string)
	if !ok {
		return ""
	}
	return cidr
}
