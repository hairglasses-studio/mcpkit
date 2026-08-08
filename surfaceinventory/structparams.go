package surfaceinventory

import (
	"go/ast"
	"go/token"
	"reflect"
	"strconv"
	"strings"
)

// Struct-based parameter resolution for mcpkit TypedHandler tools. The input
// parameters of `handler.TypedHandler[In, Out]("name","desc",fn)` live in the
// `In` struct's fields + json/jsonschema tags, not inline at the call site.
// We resolve them with a same-DIRECTORY ast struct index (Go packages are 1:1
// with directories) built from the files the extractor already parses — no
// go/packages, no type checker, no subprocess.
//
// Scope (documented skips, per the type-resolution research — "skip, don't
// guess"): unqualified same-package `In` types only. Cross-package/qualified
// types, external-module types, type aliases, generic user-type instantiations,
// and embedded fields are not resolved (params stay nil for that tool).
//
// Tag grammar is the invopop/jsonschema one TypedHandler uses (NOT the
// official-SDK AddTool grammar, where the whole tag is the description — that
// path is a separate follow-up). Required = json has no `,omitempty` OR the
// jsonschema tag carries a bare `required`.

// structIndex maps struct type names to declarations within one package dir.
type structIndex struct {
	byName map[string]*ast.StructType
}

func newStructIndex() *structIndex { return &structIndex{byName: map[string]*ast.StructType{}} }

// indexFile records every top-level `type Name struct{…}` in a file.
func (si *structIndex) indexFile(file *ast.File) {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.TypeParams != nil { // skip generic type declarations
				continue
			}
			if st, ok := ts.Type.(*ast.StructType); ok {
				si.byName[ts.Name.Name] = st
			}
		}
	}
}

// firstTypeArg returns the first type argument of a generic call target
// (TypedHandler[In, Out] → In), or nil for a non-generic call.
func firstTypeArg(fun ast.Expr) ast.Expr {
	switch f := fun.(type) {
	case *ast.IndexExpr:
		return f.Index
	case *ast.IndexListExpr:
		if len(f.Indices) > 0 {
			return f.Indices[0]
		}
	}
	return nil
}

// resolveStructParams resolves a TypedHandler call's input struct to its
// parameters. Returns nil (not empty) when the type can't be resolved from the
// same-directory index, keeping the "unknown ≠ param-less" distinction.
func resolveStructParams(call *ast.CallExpr, si *structIndex) []ToolParam {
	if si == nil {
		return nil
	}
	arg := firstTypeArg(call.Fun)
	if arg == nil {
		return nil
	}
	if star, ok := arg.(*ast.StarExpr); ok { // TypedHandler[*In, Out]
		arg = star.X
	}
	id, ok := arg.(*ast.Ident) // unqualified same-package only
	if !ok {
		return nil
	}
	st, ok := si.byName[id.Name]
	if !ok {
		return nil
	}
	return structFieldParams(st)
}

func structFieldParams(st *ast.StructType) []ToolParam {
	var params []ToolParam
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 { // embedded/anonymous — skip
			continue
		}
		jsonName, omitempty, jsonSchema := parseFieldTags(field.Tag)
		if jsonName == "-" {
			continue
		}
		hasDesc, hasEnum, bareRequired, skip := parseInvopopSchemaTag(jsonSchema)
		if skip {
			continue
		}
		for _, nm := range field.Names {
			pname := nm.Name
			if jsonName != "" && len(field.Names) == 1 {
				pname = jsonName
			}
			params = append(params, ToolParam{
				Name:           pname,
				Type:           goTypeName(field.Type),
				HasDescription: hasDesc,
				HasEnum:        hasEnum,
				Required:       bareRequired || !omitempty,
			})
		}
	}
	return params
}

// parseFieldTags returns the json field name, whether json has `,omitempty`,
// and the raw jsonschema tag value.
func parseFieldTags(tag *ast.BasicLit) (jsonName string, omitempty bool, jsonSchema string) {
	if tag == nil {
		return "", false, ""
	}
	raw, err := strconv.Unquote(tag.Value)
	if err != nil {
		return "", false, ""
	}
	st := reflect.StructTag(raw)
	jsonSchema = st.Get("jsonschema")
	parts := strings.Split(st.Get("json"), ",")
	if len(parts) > 0 {
		jsonName = parts[0]
	}
	for _, p := range parts[1:] {
		if p == "omitempty" {
			omitempty = true
		}
	}
	return jsonName, omitempty, jsonSchema
}

// parseInvopopSchemaTag parses the invopop/jsonschema tag grammar for the
// signals param_quality needs: a non-empty description=, any enum= (repeatable),
// a bare `required`, and a bare `-` (skip field).
func parseInvopopSchemaTag(tag string) (hasDesc, hasEnum, bareRequired, skip bool) {
	if tag == "" {
		return
	}
	for _, tok := range splitUnescapedCommas(tag) {
		switch {
		case tok == "-":
			skip = true
		case tok == "required":
			bareRequired = true
		case strings.HasPrefix(tok, "enum="):
			hasEnum = true
		case strings.HasPrefix(tok, "description=") && len(tok) > len("description="):
			hasDesc = true
		}
	}
	return
}

// splitUnescapedCommas splits on commas not preceded by a backslash (invopop's
// tag-value splitting), so a comma inside a description= value is preserved.
func splitUnescapedCommas(s string) []string {
	var out []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			cur.WriteByte(s[i+1])
			i++
			continue
		}
		if s[i] == ',' {
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(s[i])
	}
	out = append(out, cur.String())
	return out
}

// goTypeName maps a Go field type to the coarse MCP param type label.
func goTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		switch t.Name {
		case "string":
			return "String"
		case "bool":
			return "Boolean"
		case "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64",
			"float32", "float64":
			return "Number"
		}
		return "Object"
	case *ast.ArrayType:
		return "Array"
	case *ast.MapType, *ast.StructType, *ast.SelectorExpr:
		return "Object"
	case *ast.StarExpr:
		return goTypeName(t.X)
	}
	return ""
}
