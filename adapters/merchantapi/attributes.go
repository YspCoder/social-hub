package merchantapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

var knownProductAttributeFields = map[string]struct{}{
	"title": {}, "description": {}, "link": {}, "canonicalLink": {}, "imageLink": {},
	"additionalImageLinks": {}, "videoLinks": {}, "availability": {}, "availabilityDate": {},
	"condition": {}, "price": {}, "salePrice": {}, "brand": {}, "gtins": {}, "mpn": {},
	"googleProductCategory": {}, "productTypes": {}, "itemGroupId": {}, "color": {}, "size": {},
	"adult": {}, "customLabel0": {}, "customLabel1": {}, "customLabel2": {}, "customLabel3": {},
	"customLabel4": {}, "includedDestinations": {}, "excludedDestinations": {}, "pause": {},
	"expirationDate": {},
}

func (attributes ProductAttributes) MarshalJSON() ([]byte, error) {
	type alias ProductAttributes
	base, err := json.Marshal(alias(attributes))
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(base, &fields); err != nil {
		return nil, err
	}
	for name, value := range attributes.Extra {
		if knownProductAttributeField(name) {
			return nil, fmt.Errorf("merchantapi: extra product attribute %q overrides a typed field", name)
		}
		if !validJSONFieldName(name) || !validJSONValue(value) {
			return nil, fmt.Errorf("merchantapi: extra product attribute %q is invalid", name)
		}
		fields[name] = append(json.RawMessage(nil), value...)
	}
	return json.Marshal(fields)
}

func (attributes *ProductAttributes) UnmarshalJSON(data []byte) error {
	if attributes == nil || !validJSONObjectValue(data) {
		return fmt.Errorf("merchantapi: product attributes must be a JSON object")
	}
	type alias ProductAttributes
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for name := range fields {
		if knownProductAttributeField(name) {
			delete(fields, name)
		}
	}
	decoded.Extra = make(map[string]json.RawMessage, len(fields))
	for name, value := range fields {
		decoded.Extra[name] = append(json.RawMessage(nil), value...)
	}
	*attributes = ProductAttributes(decoded)
	return nil
}

func knownProductAttributeField(name string) bool {
	for known := range knownProductAttributeFields {
		if strings.EqualFold(name, known) {
			return true
		}
	}
	return false
}

func validJSONValue(value []byte) bool {
	value = bytes.TrimSpace(value)
	return len(value) > 0 && json.Valid(value)
}

func validJSONObjectValue(value []byte) bool {
	value = bytes.TrimSpace(value)
	return len(value) >= 2 && value[0] == '{' && value[len(value)-1] == '}' && json.Valid(value)
}
