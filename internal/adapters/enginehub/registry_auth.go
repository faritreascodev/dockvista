package enginehub

import (
	"encoding/base64"
	"encoding/json"
)

func encodeRegistryAuth(user, pass, server string) string {
	payload, err := json.Marshal(map[string]string{
		"username":      user,
		"password":      pass,
		"serveraddress": server,
	})
	if err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(payload)
}
