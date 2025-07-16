package utils

import "encoding/json"

func ToByte(msg interface{}) []byte {
	json_bytes, _ := json.Marshal(msg)
	return json_bytes
}
