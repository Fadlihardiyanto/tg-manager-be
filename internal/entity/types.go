package entity

import (
	"database/sql/driver"
	json "github.com/bytedance/sonic"
	"fmt"
)

// JSONMap adalah custom type untuk kolom JSONB di PostgreSQL
type JSONMap map[string]interface{}

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	b, err := json.Marshal(j)
	return string(b), err
}

func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = JSONMap{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("JSONMap: cannot scan type %T", value)
	}
	return json.Unmarshal(bytes, j)
}
