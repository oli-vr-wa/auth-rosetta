package api

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"
)

// decodeJSON parses exactly one JSON value from r into dst. Go 1.27's v2
// decoder is strict by default: invalid UTF-8, duplicate names and trailing
// data are errors. The alias avoids a clash with the v1 package that
// respond.go still uses for output.
func decodeJSON(r io.Reader, dst any) error {
	return jsonv2.UnmarshalRead(r, dst)
}

func decodeAuthBody[T any](r io.Reader) (value T, badText bool, err error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return value, false, err
	}
	strictErr := jsonv2.Unmarshal(data, &value)
	if strictErr == nil {
		return value, false, nil
	}

	var lenient T
	if jsonv2.Unmarshal(data, &lenient, jsontext.AllowInvalidUTF8(true)) != nil {
		return value, false, errors.New("invalid JSON body")
	}
	return lenient, true, nil
}
