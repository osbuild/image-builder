package openapi3

import (
	"context"
	"encoding/json"
)

// Contact is specified by OpenAPI/Swagger standard version 3.
// See https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.3.md#contact-object
type Contact struct {
	Extensions map[string]interface{} `json:"-" yaml:"-"`

	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	URL   string `json:"url,omitempty" yaml:"url,omitempty"`
	Email string `json:"email,omitempty" yaml:"email,omitempty"`
}

// MarshalJSON returns the JSON encoding of Contact.
func (contact Contact) MarshalJSON() ([]byte, error) {
	m := make(map[string]interface{}, 3+len(contact.Extensions))
	for k, v := range contact.Extensions {
		m[k] = v
	}
	if x := contact.Name; x != "" {
		m["name"] = x
	}
	if x := contact.URL; x != "" {
		m["url"] = x
	}
	if x := contact.Email; x != "" {
		m["email"] = x
	}
	return json.Marshal(m)
}

// UnmarshalJSON sets Contact to a copy of data.
func (contact *Contact) UnmarshalJSON(data []byte) error {
	type ContactBis Contact
	var x ContactBis
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	_ = json.Unmarshal(data, &x.Extensions)
	delete(x.Extensions, "name")
	delete(x.Extensions, "url")
	delete(x.Extensions, "email")
	*contact = Contact(x)
	return nil
}

// Validate returns an error if Contact does not comply with the OpenAPI spec.
func (contact *Contact) Validate(ctx context.Context, opts ...ValidationOption) error {
	ctx = WithValidationOptions(ctx, opts...)

	return validateExtensions(ctx, contact.Extensions)
}
