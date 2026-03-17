package openapi

import (
	"fmt"
	"strconv"
)

// Parse converts a map[string]any (parsed from YAML or JSON) into a typed OpenAPISpec.
func Parse(data map[string]any) (*OpenAPISpec, error) {
	return parseSpec(data, "")
}

// parseSpec parses the root OpenAPI spec object.
func parseSpec(data map[string]any, path string) (*OpenAPISpec, error) {
	if data == nil {
		return nil, &ParseError{Path: path, Message: "spec is nil"}
	}

	spec := &OpenAPISpec{}

	// Parse openapi version
	if v, ok := getString(data, "openapi"); ok {
		spec.OpenAPI = v
	} else {
		return nil, &ParseError{Path: path + ".openapi", Message: "openapi version is required"}
	}

	// Parse info
	if infoData, ok := getMap(data, "info"); ok {
		info, err := parseInfo(infoData, path+".info")
		if err != nil {
			return nil, err
		}
		spec.Info = *info
	} else {
		return nil, &ParseError{Path: path + ".info", Message: "info is required"}
	}

	// Parse servers
	if serversData, ok := getSlice(data, "servers"); ok {
		servers, err := parseServers(serversData, path+".servers")
		if err != nil {
			return nil, err
		}
		spec.Servers = servers
	}

	// Parse paths
	if pathsData, ok := getMap(data, "paths"); ok {
		paths, err := parsePaths(pathsData, path+".paths")
		if err != nil {
			return nil, err
		}
		spec.Paths = paths
	} else {
		spec.Paths = make(map[string]*PathItem)
	}

	// Parse components
	if compData, ok := getMap(data, "components"); ok {
		components, err := parseComponents(compData, path+".components")
		if err != nil {
			return nil, err
		}
		spec.Components = components
	}

	// Parse tags
	if tagsData, ok := getSlice(data, "tags"); ok {
		tags, err := parseTags(tagsData, path+".tags")
		if err != nil {
			return nil, err
		}
		spec.Tags = tags
	}

	return spec, nil
}

// parseInfo parses the Info object.
func parseInfo(data map[string]any, path string) (*Info, error) {
	info := &Info{}

	if v, ok := getString(data, "title"); ok {
		info.Title = v
	} else {
		return nil, &ParseError{Path: path + ".title", Message: "title is required"}
	}

	if v, ok := getString(data, "version"); ok {
		info.Version = v
	} else {
		return nil, &ParseError{Path: path + ".version", Message: "version is required"}
	}

	if v, ok := getString(data, "description"); ok {
		info.Description = v
	}

	if v, ok := getString(data, "termsOfService"); ok {
		info.TermsOfService = v
	}

	if contactData, ok := getMap(data, "contact"); ok {
		info.Contact = parseContact(contactData)
	}

	if licenseData, ok := getMap(data, "license"); ok {
		license, err := parseLicense(licenseData, path+".license")
		if err != nil {
			return nil, err
		}
		info.License = license
	}

	return info, nil
}

// parseContact parses the Contact object.
func parseContact(data map[string]any) *Contact {
	contact := &Contact{}
	if v, ok := getString(data, "name"); ok {
		contact.Name = v
	}
	if v, ok := getString(data, "url"); ok {
		contact.URL = v
	}
	if v, ok := getString(data, "email"); ok {
		contact.Email = v
	}
	return contact
}

// parseLicense parses the License object.
func parseLicense(data map[string]any, path string) (*License, error) {
	license := &License{}

	if v, ok := getString(data, "name"); ok {
		license.Name = v
	} else {
		return nil, &ParseError{Path: path + ".name", Message: "license name is required"}
	}

	if v, ok := getString(data, "url"); ok {
		license.URL = v
	}

	return license, nil
}

// parseServers parses a slice of Server objects.
func parseServers(data []any, path string) ([]Server, error) {
	servers := make([]Server, 0, len(data))

	for i, item := range data {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		itemMap, ok := item.(map[string]any)
		if !ok {
			return nil, &ParseError{Path: itemPath, Message: "server must be an object"}
		}

		server, err := parseServer(itemMap, itemPath)
		if err != nil {
			return nil, err
		}
		servers = append(servers, *server)
	}

	return servers, nil
}

// parseServer parses a Server object.
func parseServer(data map[string]any, path string) (*Server, error) {
	server := &Server{}

	if v, ok := getString(data, "url"); ok {
		server.URL = v
	} else {
		return nil, &ParseError{Path: path + ".url", Message: "server url is required"}
	}

	if v, ok := getString(data, "description"); ok {
		server.Description = v
	}

	if varsData, ok := getMap(data, "variables"); ok {
		vars := make(map[string]*ServerVariable)
		for name, varData := range varsData {
			varMap, ok := varData.(map[string]any)
			if !ok {
				return nil, &ParseError{Path: path + ".variables." + name, Message: "variable must be an object"}
			}
			variable := parseServerVariable(varMap)
			vars[name] = variable
		}
		server.Variables = vars
	}

	return server, nil
}

// parseServerVariable parses a ServerVariable object.
func parseServerVariable(data map[string]any) *ServerVariable {
	variable := &ServerVariable{}

	if v, ok := getString(data, "default"); ok {
		variable.Default = v
	}

	if v, ok := getString(data, "description"); ok {
		variable.Description = v
	}

	if enumData, ok := getSlice(data, "enum"); ok {
		enum := make([]string, 0, len(enumData))
		for _, e := range enumData {
			if s, ok := e.(string); ok {
				enum = append(enum, s)
			}
		}
		variable.Enum = enum
	}

	return variable
}

// parsePaths parses the paths object.
func parsePaths(data map[string]any, path string) (map[string]*PathItem, error) {
	paths := make(map[string]*PathItem, len(data))

	for pathKey, pathData := range data {
		itemPath := path + "." + pathKey
		pathMap, ok := pathData.(map[string]any)
		if !ok {
			return nil, &ParseError{Path: itemPath, Message: "path item must be an object"}
		}

		pathItem, err := parsePathItem(pathMap, itemPath)
		if err != nil {
			return nil, err
		}
		paths[pathKey] = pathItem
	}

	return paths, nil
}

// parsePathItem parses a PathItem object.
func parsePathItem(data map[string]any, path string) (*PathItem, error) {
	item := &PathItem{}

	// Check for $ref
	if v, ok := getString(data, "$ref"); ok {
		item.Ref = v
		// If it's just a ref, we may not have other fields
		if len(data) == 1 {
			return item, nil
		}
	}

	if v, ok := getString(data, "summary"); ok {
		item.Summary = v
	}

	if v, ok := getString(data, "description"); ok {
		item.Description = v
	}

	// Parse operations
	methods := map[string]**Operation{
		"get":     &item.Get,
		"put":     &item.Put,
		"post":    &item.Post,
		"delete":  &item.Delete,
		"options": &item.Options,
		"head":    &item.Head,
		"patch":   &item.Patch,
		"trace":   &item.Trace,
	}

	for method, target := range methods {
		if opData, ok := getMap(data, method); ok {
			op, err := parseOperation(opData, path+"."+method)
			if err != nil {
				return nil, err
			}
			*target = op
		}
	}

	// Parse parameters
	if paramsData, ok := getSlice(data, "parameters"); ok {
		params, err := parseParameters(paramsData, path+".parameters")
		if err != nil {
			return nil, err
		}
		item.Parameters = params
	}

	// Parse servers
	if serversData, ok := getSlice(data, "servers"); ok {
		servers, err := parseServers(serversData, path+".servers")
		if err != nil {
			return nil, err
		}
		item.Servers = servers
	}

	return item, nil
}

// parseOperation parses an Operation object.
func parseOperation(data map[string]any, path string) (*Operation, error) {
	op := &Operation{
		Responses: make(map[string]*Response),
	}

	if v, ok := getString(data, "operationId"); ok {
		op.OperationID = v
	}

	if v, ok := getString(data, "summary"); ok {
		op.Summary = v
	}

	if v, ok := getString(data, "description"); ok {
		op.Description = v
	}

	if v, ok := getBool(data, "deprecated"); ok {
		op.Deprecated = v
	}

	// Parse tags
	if tagsData, ok := getSlice(data, "tags"); ok {
		tags := make([]string, 0, len(tagsData))
		for _, t := range tagsData {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
		op.Tags = tags
	}

	// Parse parameters
	if paramsData, ok := getSlice(data, "parameters"); ok {
		params, err := parseParameters(paramsData, path+".parameters")
		if err != nil {
			return nil, err
		}
		op.Parameters = params
	}

	// Parse requestBody
	if bodyData, ok := getMap(data, "requestBody"); ok {
		body, err := parseRequestBody(bodyData, path+".requestBody")
		if err != nil {
			return nil, err
		}
		op.RequestBody = body
	}

	// Parse responses (required)
	if respData, ok := getMap(data, "responses"); ok {
		for code, respValue := range respData {
			respMap, ok := respValue.(map[string]any)
			if !ok {
				return nil, &ParseError{Path: path + ".responses." + code, Message: "response must be an object"}
			}
			resp, err := parseResponse(respMap, path+".responses."+code)
			if err != nil {
				return nil, err
			}
			op.Responses[code] = resp
		}
	}

	return op, nil
}

// parseParameters parses a slice of Parameter objects.
func parseParameters(data []any, path string) ([]*Parameter, error) {
	params := make([]*Parameter, 0, len(data))

	for i, item := range data {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		itemMap, ok := item.(map[string]any)
		if !ok {
			return nil, &ParseError{Path: itemPath, Message: "parameter must be an object"}
		}

		param, err := parseParameter(itemMap, itemPath)
		if err != nil {
			return nil, err
		}
		params = append(params, param)
	}

	return params, nil
}

// parseParameter parses a Parameter object.
func parseParameter(data map[string]any, path string) (*Parameter, error) {
	param := &Parameter{}

	// Check for $ref
	if v, ok := getString(data, "$ref"); ok {
		param.Ref = v
		return param, nil
	}

	if v, ok := getString(data, "name"); ok {
		param.Name = v
	} else {
		return nil, &ParseError{Path: path + ".name", Message: "parameter name is required"}
	}

	if v, ok := getString(data, "in"); ok {
		param.In = v
	} else {
		return nil, &ParseError{Path: path + ".in", Message: "parameter 'in' is required"}
	}

	if v, ok := getString(data, "description"); ok {
		param.Description = v
	}

	if v, ok := getBool(data, "required"); ok {
		param.Required = v
	}

	if v, ok := getBool(data, "deprecated"); ok {
		param.Deprecated = v
	}

	if v, ok := getBool(data, "allowEmptyValue"); ok {
		param.AllowEmptyValue = v
	}

	// Parse schema
	if schemaData, ok := getMap(data, "schema"); ok {
		schema, err := parseSchema(schemaData, path+".schema")
		if err != nil {
			return nil, err
		}
		param.Schema = schema
	}

	return param, nil
}

// parseRequestBody parses a RequestBody object.
func parseRequestBody(data map[string]any, path string) (*RequestBody, error) {
	body := &RequestBody{}

	// Check for $ref
	if v, ok := getString(data, "$ref"); ok {
		body.Ref = v
		return body, nil
	}

	if v, ok := getString(data, "description"); ok {
		body.Description = v
	}

	if v, ok := getBool(data, "required"); ok {
		body.Required = v
	}

	// Parse content
	if contentData, ok := getMap(data, "content"); ok {
		content := make(map[string]*MediaType)
		for mediaType, mtData := range contentData {
			mtMap, ok := mtData.(map[string]any)
			if !ok {
				return nil, &ParseError{Path: path + ".content." + mediaType, Message: "media type must be an object"}
			}
			mt, err := parseMediaType(mtMap, path+".content."+mediaType)
			if err != nil {
				return nil, err
			}
			content[mediaType] = mt
		}
		body.Content = content
	}

	return body, nil
}

// parseResponse parses a Response object.
func parseResponse(data map[string]any, path string) (*Response, error) {
	resp := &Response{}

	// Check for $ref
	if v, ok := getString(data, "$ref"); ok {
		resp.Ref = v
		return resp, nil
	}

	if v, ok := getString(data, "description"); ok {
		resp.Description = v
	}

	// Parse content
	if contentData, ok := getMap(data, "content"); ok {
		content := make(map[string]*MediaType)
		for mediaType, mtData := range contentData {
			mtMap, ok := mtData.(map[string]any)
			if !ok {
				return nil, &ParseError{Path: path + ".content." + mediaType, Message: "media type must be an object"}
			}
			mt, err := parseMediaType(mtMap, path+".content."+mediaType)
			if err != nil {
				return nil, err
			}
			content[mediaType] = mt
		}
		resp.Content = content
	}

	return resp, nil
}

// parseMediaType parses a MediaType object.
func parseMediaType(data map[string]any, path string) (*MediaType, error) {
	mt := &MediaType{}

	// Parse schema
	if schemaData, ok := getMap(data, "schema"); ok {
		schema, err := parseSchema(schemaData, path+".schema")
		if err != nil {
			return nil, err
		}
		mt.Schema = schema
	}

	// Example and Examples parsing would go here
	if v, ok := data["example"]; ok {
		mt.Example = v
	}

	return mt, nil
}

// parseSchema parses a SchemaObject.
func parseSchema(data map[string]any, path string) (*SchemaObject, error) {
	schema := &SchemaObject{}

	// Check for $ref
	if v, ok := getString(data, "$ref"); ok {
		schema.Ref = v
		return schema, nil
	}

	// Parse basic fields
	if v, ok := getString(data, "title"); ok {
		schema.Title = v
	}

	if v, ok := getString(data, "type"); ok {
		schema.Type = v
	}

	if v, ok := getString(data, "description"); ok {
		schema.Description = v
	}

	if v, ok := getString(data, "format"); ok {
		schema.Format = v
	}

	if v, ok := getBool(data, "nullable"); ok {
		schema.Nullable = v
	}

	if v, ok := getBool(data, "readOnly"); ok {
		schema.ReadOnly = v
	}

	if v, ok := getBool(data, "writeOnly"); ok {
		schema.WriteOnly = v
	}

	if v, ok := getBool(data, "deprecated"); ok {
		schema.Deprecated = v
	}

	if v, ok := getBool(data, "uniqueItems"); ok {
		schema.UniqueItems = v
	}

	// Parse numeric fields
	if v, ok := getFloat(data, "multipleOf"); ok {
		schema.MultipleOf = v
	}

	if v, ok := getFloat(data, "maximum"); ok {
		schema.Maximum = v
	}

	if v, ok := getFloat(data, "minimum"); ok {
		schema.Minimum = v
	}

	if v, ok := getBool(data, "exclusiveMaximum"); ok {
		schema.ExclusiveMaximum = v
	}

	if v, ok := getBool(data, "exclusiveMinimum"); ok {
		schema.ExclusiveMinimum = v
	}

	// Parse integer fields
	if v, ok := getInt(data, "maxLength"); ok {
		schema.MaxLength = v
	}

	if v, ok := getInt(data, "minLength"); ok {
		schema.MinLength = v
	}

	if v, ok := getInt(data, "maxItems"); ok {
		schema.MaxItems = v
	}

	if v, ok := getInt(data, "minItems"); ok {
		schema.MinItems = v
	}

	if v, ok := getInt(data, "maxProperties"); ok {
		schema.MaxProperties = v
	}

	if v, ok := getInt(data, "minProperties"); ok {
		schema.MinProperties = v
	}

	// Parse string fields
	if v, ok := getString(data, "pattern"); ok {
		schema.Pattern = v
	}

	// Parse required array
	if reqData, ok := getSlice(data, "required"); ok {
		required := make([]string, 0, len(reqData))
		for _, r := range reqData {
			if s, ok := r.(string); ok {
				required = append(required, s)
			}
		}
		schema.Required = required
	}

	// Parse enum array
	if enumData, ok := getSlice(data, "enum"); ok {
		schema.Enum = enumData
	}

	// Parse items (for arrays)
	if itemsData, ok := getMap(data, "items"); ok {
		items, err := parseSchema(itemsData, path+".items")
		if err != nil {
			return nil, err
		}
		schema.Items = items
	}

	// Parse properties (for objects)
	if propsData, ok := getMap(data, "properties"); ok {
		props := make(map[string]*SchemaObject)
		for propName, propValue := range propsData {
			propMap, ok := propValue.(map[string]any)
			if !ok {
				return nil, &ParseError{Path: path + ".properties." + propName, Message: "property must be an object"}
			}
			prop, err := parseSchema(propMap, path+".properties."+propName)
			if err != nil {
				return nil, err
			}
			props[propName] = prop
		}
		schema.Properties = props
	}

	// Parse allOf
	if allOfData, ok := getSlice(data, "allOf"); ok {
		allOf, err := parseSchemaSlice(allOfData, path+".allOf")
		if err != nil {
			return nil, err
		}
		schema.AllOf = allOf
	}

	// Parse oneOf
	if oneOfData, ok := getSlice(data, "oneOf"); ok {
		oneOf, err := parseSchemaSlice(oneOfData, path+".oneOf")
		if err != nil {
			return nil, err
		}
		schema.OneOf = oneOf
	}

	// Parse anyOf
	if anyOfData, ok := getSlice(data, "anyOf"); ok {
		anyOf, err := parseSchemaSlice(anyOfData, path+".anyOf")
		if err != nil {
			return nil, err
		}
		schema.AnyOf = anyOf
	}

	// Parse not
	if notData, ok := getMap(data, "not"); ok {
		not, err := parseSchema(notData, path+".not")
		if err != nil {
			return nil, err
		}
		schema.Not = not
	}

	return schema, nil
}

// parseSchemaSlice parses a slice of SchemaObjects.
func parseSchemaSlice(data []any, path string) ([]*SchemaObject, error) {
	schemas := make([]*SchemaObject, 0, len(data))
	for i, item := range data {
		itemMap, ok := item.(map[string]any)
		if !ok {
			return nil, &ParseError{Path: fmt.Sprintf("%s[%d]", path, i), Message: "schema must be an object"}
		}
		schema, err := parseSchema(itemMap, fmt.Sprintf("%s[%d]", path, i))
		if err != nil {
			return nil, err
		}
		schemas = append(schemas, schema)
	}
	return schemas, nil
}

// parseComponents parses the Components object.
func parseComponents(data map[string]any, path string) (*Components, error) {
	comp := &Components{}

	// Parse schemas
	if schemasData, ok := getMap(data, "schemas"); ok {
		schemas := make(map[string]*SchemaObject)
		for name, schemaData := range schemasData {
			schemaMap, ok := schemaData.(map[string]any)
			if !ok {
				return nil, &ParseError{Path: path + ".schemas." + name, Message: "schema must be an object"}
			}
			schema, err := parseSchema(schemaMap, path+".schemas."+name)
			if err != nil {
				return nil, err
			}
			schemas[name] = schema
		}
		comp.Schemas = schemas
	}

	// Parse responses
	if respData, ok := getMap(data, "responses"); ok {
		responses := make(map[string]*Response)
		for name, rData := range respData {
			rMap, ok := rData.(map[string]any)
			if !ok {
				return nil, &ParseError{Path: path + ".responses." + name, Message: "response must be an object"}
			}
			resp, err := parseResponse(rMap, path+".responses."+name)
			if err != nil {
				return nil, err
			}
			responses[name] = resp
		}
		comp.Responses = responses
	}

	// Parse parameters
	if paramsData, ok := getMap(data, "parameters"); ok {
		params := make(map[string]*Parameter)
		for name, pData := range paramsData {
			pMap, ok := pData.(map[string]any)
			if !ok {
				return nil, &ParseError{Path: path + ".parameters." + name, Message: "parameter must be an object"}
			}
			param, err := parseParameter(pMap, path+".parameters."+name)
			if err != nil {
				return nil, err
			}
			params[name] = param
		}
		comp.Parameters = params
	}

	// Parse request bodies
	if bodiesData, ok := getMap(data, "requestBodies"); ok {
		bodies := make(map[string]*RequestBody)
		for name, bData := range bodiesData {
			bMap, ok := bData.(map[string]any)
			if !ok {
				return nil, &ParseError{Path: path + ".requestBodies." + name, Message: "request body must be an object"}
			}
			body, err := parseRequestBody(bMap, path+".requestBodies."+name)
			if err != nil {
				return nil, err
			}
			bodies[name] = body
		}
		comp.RequestBodies = bodies
	}

	return comp, nil
}

// parseTags parses a slice of Tag objects.
func parseTags(data []any, path string) ([]Tag, error) {
	tags := make([]Tag, 0, len(data))
	for i, item := range data {
		itemMap, ok := item.(map[string]any)
		if !ok {
			return nil, &ParseError{Path: fmt.Sprintf("%s[%d]", path, i), Message: "tag must be an object"}
		}

		tag := Tag{}
		if v, ok := getString(itemMap, "name"); ok {
			tag.Name = v
		}
		if v, ok := getString(itemMap, "description"); ok {
			tag.Description = v
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

// Helper functions for type-safe map access

func getString(data map[string]any, key string) (string, bool) {
	if v, ok := data[key]; ok {
		switch val := v.(type) {
		case string:
			return val, true
		case []byte:
			return string(val), true
		default:
			return "", false
		}
	}
	return "", false
}

func getBool(data map[string]any, key string) (bool, bool) {
	if v, ok := data[key]; ok {
		switch val := v.(type) {
		case bool:
			return val, true
		default:
			return false, false
		}
	}
	return false, false
}

func getInt(data map[string]any, key string) (int, bool) {
	if v, ok := data[key]; ok {
		switch val := v.(type) {
		case int:
			return val, true
		case int64:
			return int(val), true
		case float64:
			return int(val), true
		case string:
			if i, err := strconv.Atoi(val); err == nil {
				return i, true
			}
		}
	}
	return 0, false
}

func getFloat(data map[string]any, key string) (float64, bool) {
	if v, ok := data[key]; ok {
		switch val := v.(type) {
		case float64:
			return val, true
		case int:
			return float64(val), true
		case int64:
			return float64(val), true
		case string:
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				return f, true
			}
		}
	}
	return 0, false
}

func getMap(data map[string]any, key string) (map[string]any, bool) {
	if v, ok := data[key]; ok {
		if m, ok := v.(map[string]any); ok {
			return m, true
		}
	}
	return nil, false
}

func getSlice(data map[string]any, key string) ([]any, bool) {
	if v, ok := data[key]; ok {
		if s, ok := v.([]any); ok {
			return s, true
		}
	}
	return nil, false
}
