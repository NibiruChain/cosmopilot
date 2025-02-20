package utils

import (
	"bytes"

	"github.com/BurntSushi/toml"
	"github.com/RaveNoX/go-jsonmerge"
)

func TomlDecode(data string) (interface{}, error) {
	var out interface{}
	_, err := toml.Decode(data, &out)
	return out, err
}

func Merge(data, patch interface{}) (interface{}, error) {
	out, info := jsonmerge.Merge(data, patch)
	if len(info.Errors) > 0 {
		return nil, info.Errors[0]
	}
	return out, nil
}

func TomlEncode(in interface{}) (string, error) {
	buf := new(bytes.Buffer)
	if err := toml.NewEncoder(buf).Encode(in); err != nil {
		return "", err
	}
	return buf.String(), nil
}
