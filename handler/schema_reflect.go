// schema_reflect.go -- SDK-NEUTRAL JSON Schema derivation from Go types.
//
// This file carries NO build tag on purpose. It is the single source of
// truth for turning a Go input/output struct into JSON Schema properties,
// shared verbatim by typed.go (mcp-go) and typed_official.go (official
// go-sdk). Sharing one code path is what makes "both build tags advertise
// the same schema" a structural property rather than a promise that two
// parallel implementations have to keep re-earning.
//
// Why it exists (secretstudios-mcp P78.38, 2026-08-16). typed_official.go
// used to derive schemas by marshaling a ZERO VALUE of the type and
// enumerating the resulting JSON object's keys. Every field tagged
// `json:",omitempty"` is by definition absent from a zero value's
// marshaling, so every such field silently vanished from the advertised
// schema. Measured across secretstudios-mcp's real 233-tool registry, that
// dropped 1462 input properties to 145 -- its two largest polymorphic tools,
// wan_inspect (24 properties) and switch_inspect (23), advertised ZERO, and
// all 1419 property descriptions and all 106 required-field entries were
// lost outright. A caller had no wire-level way to discover or supply any
// optional parameter. The zero-value approach also could not recover
// descriptions, enums, defaults, or required-ness at all, and inferred types
// from the marshaled zero value (so a *bool read back as JSON null was typed
// "string"). It is not fixable in place; reflection over the type is the
// only way to see a field that a zero value omits.
package handler

import "github.com/invopop/jsonschema"

// reflectSchemaProperties reflects over T and returns its top-level JSON
// Schema properties plus the required-field list. DoNotReference inlines
// nested definitions rather than emitting $ref/$defs, so the returned map is
// self-contained -- consumers (and MCP clients) never have to resolve a
// reference against a document we don't ship.
//
// A nil/absent Properties (T is not a struct, e.g. a map or slice type)
// yields an empty, non-nil map rather than nil, so callers can always range
// over the result. required is nil when empty so it can be omitted from the
// emitted schema instead of serializing as [].
func reflectSchemaProperties[T any]() (properties map[string]any, required []string) {
	r := jsonschema.Reflector{DoNotReference: true}
	schema := r.Reflect(new(T))

	properties = make(map[string]any)
	if schema.Properties != nil {
		for pair := schema.Properties.Oldest(); pair != nil; pair = pair.Next() {
			properties[pair.Key] = schemaToMap(pair.Value)
		}
	}
	if len(schema.Required) > 0 {
		required = schema.Required
	}
	return properties, required
}

// reflectSchemaMap returns T's schema as a plain JSON Schema object map --
// the shape the official SDK wants, since its Tool.InputSchema and
// Tool.OutputSchema are both `any`.
//
// "properties" is emitted unconditionally (as {} when T has no fields) so
// the advertised schema is always a well-formed object schema, and "type"
// is always "object" because the official SDK's Server.AddTool panics on an
// input schema whose type is anything else.
func reflectSchemaMap[T any]() map[string]any {
	properties, required := reflectSchemaProperties[T]()
	m := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}

// schemaToMap converts a reflected jsonschema.Schema to the map form both
// SDKs' schema fields carry. Recursive: nested object properties and array
// item schemas are converted the same way.
func schemaToMap(s *jsonschema.Schema) map[string]any {
	m := make(map[string]any)

	if s.Type != "" {
		m["type"] = s.Type
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	if s.Format != "" {
		m["format"] = s.Format
	}
	if len(s.Enum) > 0 {
		m["enum"] = s.Enum
	}
	if s.Default != nil {
		m["default"] = s.Default
	}

	// Handle array items
	if s.Items != nil && s.Items.Type != "" {
		m["items"] = schemaToMap(s.Items)
	}

	// Handle nested object properties
	if s.Properties != nil && s.Properties.Len() > 0 {
		props := make(map[string]any)
		for pair := s.Properties.Oldest(); pair != nil; pair = pair.Next() {
			props[pair.Key] = schemaToMap(pair.Value)
		}
		m["properties"] = props
		if len(s.Required) > 0 {
			m["required"] = s.Required
		}
	}

	return m
}
