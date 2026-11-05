package kit

import (
	"encoding/json"
	"io"
)

func valuesOn(in io.Reader) (map[string]string, error) {
	values := map[string]string{}
	if in == nil {
		return values, nil
	}
	if err := json.NewDecoder(in).Decode(&values); err != nil {
		return nil, usage("the form's answers on stdin are not a JSON object", "run it on a terminal")
	}
	return values, nil
}
