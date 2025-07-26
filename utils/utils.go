package utils

import (
	"encoding/json"
	"fmt"
	"reflect"
)

func ToByte(msg interface{}) []byte {
	json_bytes, _ := json.Marshal(msg)
	return json_bytes
}

func MapToStruct(data map[string]interface{}, result interface{}) error {
	if err := checkStruct(data, result); err != nil {
		return err
	}
	// map을 JSON으로 변환
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// JSON을 struct로 변환
	if err := json.Unmarshal(jsonData, result); err != nil {
		return err
	}

	t := reflect.TypeOf(result)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// 모든 필드가 채워졌는지 검증
	// return validateAllFieldsFilled(result)
	return nil
}

func checkStruct(a map[string]interface{}, b interface{}) error {
	t := reflect.TypeOf(b)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		var is_included bool = false
		for k, _ := range a {
			if k == t.Field(i).Name {
				is_included = true
			}
		}

		if !is_included {
			return fmt.Errorf("구조체가 일치하지 않습니다.")
		}
	}

	return nil
}

// target이 속하는 Shard를 찾아주는 함수
func FindShard(target string) int {
	return int(target[0] - 'A' + 1)
}
