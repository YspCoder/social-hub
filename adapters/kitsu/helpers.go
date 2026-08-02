package kitsu

import (
	"encoding/json"

	"social-hub/pkg/socialhub"
)

func unmarshalAttributes(source resource, output any) error {
	if len(source.Attributes) == 0 || string(source.Attributes) == "null" {
		return platformError("decode_resource", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if err := json.Unmarshal(source.Attributes, output); err != nil {
		return platformError("decode_resource", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return nil
}

func identifierRelationship(resourceType, id string) relationship {
	encoded, _ := json.Marshal(resourceIdentifier{Type: resourceType, ID: id})
	return relationship{Data: encoded}
}
