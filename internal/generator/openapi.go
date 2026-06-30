package generator

import (
	"fmt"
	"sort"
	"strings"
)

type openAPISchema struct {
	Ref    string
	Type   string
	Format string
	Items  *openAPISchema
}

type openAPIOperation struct {
	Method       string
	Path         string
	Name         string
	Tag          string
	Params       []endpointParam
	BodySchema   string
	Response     openAPISchema
	BadRequest   bool
	Unauthorized bool
	Forbidden    bool
	NotFound     bool
	ServerError  bool
	ContentType  string
	Description  string
	Security     []string
}

func buildOpenAPISpec(module string, tables []table, features FeatureOptions) string {
	operations := collectOpenAPIOperations(tables, features)
	var b strings.Builder
	b.WriteString("openapi: 3.0.3\n")
	b.WriteString("info:\n")
	fmt.Fprintf(&b, "  title: %q\n", defaultOpenAPIValue(features.OpenAPI.Title, "Generated REST API"))
	fmt.Fprintf(&b, "  version: %q\n", defaultOpenAPIValue(features.OpenAPI.Version, "0.1.0"))
	fmt.Fprintf(&b, "  description: %q\n", defaultOpenAPIValue(features.OpenAPI.Description, "Complete HTTP contract generated from "+module))
	b.WriteString("servers:\n")
	serverURL := features.OpenAPI.ServerURL
	if serverURL == "" {
		serverURL = fmt.Sprintf("http://localhost:%d", openAPIHTTPPort(features.Build.HTTPPort))
	}
	fmt.Fprintf(&b, "  - url: %q\n", serverURL)
	b.WriteString("tags:\n")
	b.WriteString("  - name: system\n")
	if features.Auth.Enabled && strings.EqualFold(features.Auth.Strategy, "jwt") {
		b.WriteString("  - name: auth\n")
	}
	for _, tbl := range tables {
		if features.Auth.Enabled && strings.EqualFold(features.Auth.Strategy, "jwt") && isAuthIdentityTable(tbl, features.Auth) {
			continue
		}
		fmt.Fprintf(&b, "  - name: %s\n", tbl.Name)
	}
	b.WriteString("paths:\n")
	currentPath := ""
	for _, operation := range operations {
		if operation.Path != currentPath {
			fmt.Fprintf(&b, "  %s:\n", operation.Path)
			currentPath = operation.Path
		}
		writeOpenAPIOperation(&b, operation)
	}
	writeOpenAPIComponents(&b, tables, features)
	return b.String()
}

func writeOpenAPIOperation(b *strings.Builder, operation openAPIOperation) {
	fmt.Fprintf(b, "    %s:\n", strings.ToLower(operation.Method))
	fmt.Fprintf(b, "      operationId: %s\n", operation.Name)
	fmt.Fprintf(b, "      tags: [%s]\n", operation.Tag)
	if operation.Description != "" {
		fmt.Fprintf(b, "      summary: %s\n", operation.Description)
	}
	if len(operation.Security) > 0 {
		b.WriteString("      security:\n")
		for _, scheme := range operation.Security {
			fmt.Fprintf(b, "        - %s: []\n", scheme)
		}
	}
	params := openAPIParams(operation)
	if len(params) > 0 {
		b.WriteString("      parameters:\n")
		for _, param := range params {
			fmt.Fprintf(b, "        - name: %s\n", param.JSONName)
			fmt.Fprintf(b, "          in: %s\n", param.Source)
			fmt.Fprintf(b, "          required: %t\n", param.Source == "path" || param.Required)
			b.WriteString("          schema:\n")
			writeOpenAPIParameterSchema(b, "            ", param)
		}
	}
	if operation.BodySchema != "" {
		b.WriteString("      requestBody:\n")
		b.WriteString("        required: true\n")
		b.WriteString("        content:\n")
		b.WriteString("          application/json:\n")
		b.WriteString("            schema:\n")
		fmt.Fprintf(b, "              $ref: '#/components/schemas/%s'\n", operation.BodySchema)
	}
	b.WriteString("      responses:\n")
	writeOpenAPIResponse(b, "200", "Successful response", operation.ContentType, operation.Response)
	if operation.BadRequest {
		writeOpenAPIErrorResponse(b, "400", "Invalid request")
	}
	if operation.Unauthorized {
		writeOpenAPIErrorResponse(b, "401", "Unauthorized")
	}
	if operation.Forbidden {
		writeOpenAPIErrorResponse(b, "403", "Forbidden")
	}
	if operation.NotFound {
		writeOpenAPIErrorResponse(b, "404", "Resource not found")
	}
	if operation.ServerError {
		writeOpenAPIErrorResponse(b, "500", "Internal server error")
	}
}

func writeOpenAPIResponse(b *strings.Builder, status, description, contentType string, schema openAPISchema) {
	fmt.Fprintf(b, "        '%s':\n", status)
	fmt.Fprintf(b, "          description: %s\n", description)
	if schema.Ref == "" && schema.Type == "" {
		return
	}
	if contentType == "" {
		contentType = "application/json"
	}
	b.WriteString("          content:\n")
	fmt.Fprintf(b, "            %s:\n", contentType)
	b.WriteString("              schema:\n")
	writeOpenAPISchema(b, "                ", schema)
}

func writeOpenAPIErrorResponse(b *strings.Builder, status, description string) {
	fmt.Fprintf(b, "        '%s':\n", status)
	fmt.Fprintf(b, "          description: %s\n", description)
	b.WriteString("          content:\n")
	b.WriteString("            application/json:\n")
	b.WriteString("              schema:\n")
	b.WriteString("                $ref: '#/components/schemas/ErrorResponse'\n")
}

func collectOpenAPIOperations(tables []table, features FeatureOptions) []openAPIOperation {
	result := []openAPIOperation{{
		Method: "GET", Path: applicationPath(features.HTTP.BasePath, "/"), Name: "GetAPIStatus", Tag: "system",
		Response: openAPISchema{Type: "string"}, ContentType: "text/plain", Description: "API status",
	}}
	if features.Auth.Enabled && strings.EqualFold(features.Auth.Strategy, "jwt") {
		result = append(result,
			openAPIOperation{Method: "POST", Path: applicationPath(features.HTTP.BasePath, "/signup"), Name: "SignUp", Tag: "auth", BodySchema: "AuthRequest", Response: openAPISchema{Ref: "SuccessResponse"}, BadRequest: true, ServerError: true, Description: "Register a user"},
			openAPIOperation{Method: "POST", Path: applicationPath(features.HTTP.BasePath, "/signin"), Name: "SignIn", Tag: "auth", BodySchema: "AuthRequest", Response: openAPISchema{Ref: "TokenResponse"}, BadRequest: true, ServerError: true, Description: "Authenticate and issue JWT token"},
		)
	}
	if features.HTTP.Health {
		result = append(result, openAPIOperation{Method: "GET", Path: applicationPath(features.HTTP.BasePath, features.HTTP.HealthPath), Name: "GetHealth", Tag: "system", Response: openAPISchema{Ref: "HealthResponse"}, Description: "Application health"})
	}
	if features.Metrics.Enabled {
		result = append(result, openAPIOperation{Method: "GET", Path: applicationPath(features.HTTP.BasePath, features.Metrics.Path), Name: "GetMetrics", Tag: "system", Response: openAPISchema{Type: "string"}, ContentType: "text/plain", Description: "Prometheus metrics"})
	}
	if features.OpenAPI.Enabled {
		result = append(result, openAPIOperation{Method: "GET", Path: applicationPath(features.HTTP.BasePath, defaultOpenAPIValue(features.OpenAPI.SpecPath, "/swagger/openapi.yaml")), Name: "GetOpenAPISpec", Tag: "system", Response: openAPISchema{Type: "string"}, ContentType: "application/yaml", Description: "OpenAPI document"})
	}
	if features.OpenAPI.Enabled && features.OpenAPI.WithUI {
		result = append(result,
			openAPIOperation{Method: "GET", Path: applicationPath(features.HTTP.BasePath, defaultOpenAPIValue(features.OpenAPI.UIPath, "/swagger/index.html")), Name: "GetSwaggerUI", Tag: "system", Response: openAPISchema{Type: "string"}, ContentType: "text/html", Description: "Swagger UI"},
		)
	}
	for _, tbl := range tables {
		if features.Auth.Enabled && strings.EqualFold(features.Auth.Strategy, "jwt") && isAuthIdentityTable(tbl, features.Auth) {
			continue
		}
		responseRef := openAPISchema{Ref: tbl.GoName + "Response"}
		if tbl.Queries.GetAll {
			result = append(result, protectOpenAPIOperation(openAPIOperation{Method: "GET", Path: applicationPath(features.HTTP.BasePath, tbl.RouteBase), Name: "GetAll" + tbl.GoPlural, Tag: tbl.Name, Response: openAPISchema{Type: "array", Items: &responseRef}, ServerError: true}, features))
		}
		if tbl.Queries.Create {
			result = append(result, protectOpenAPIOperation(openAPIOperation{Method: "POST", Path: applicationPath(features.HTTP.BasePath, tbl.RouteBase), Name: "Create" + tbl.GoName, Tag: tbl.Name, BodySchema: tbl.GoName + "Request", Response: responseRef, BadRequest: true, ServerError: true}, features))
		}
		if tbl.Queries.DeleteAll {
			result = append(result, protectOpenAPIOperation(openAPIOperation{Method: "DELETE", Path: applicationPath(features.HTTP.BasePath, tbl.RouteBase), Name: "DeleteAll" + tbl.GoPlural, Tag: tbl.Name, Response: openAPISchema{Ref: "DeletedResponse"}, ServerError: true}, features))
		}
		for _, endpoint := range tbl.Endpoints {
			operation := openAPIOperation{Method: endpoint.Method, Path: endpoint.Path, Name: endpoint.Name, Tag: tbl.Name, Params: endpoint.Params, BadRequest: len(endpoint.Params) > 0, ServerError: true}
			if len(endpoint.BodyParams) > 0 {
				operation.BodySchema = endpoint.Name + "Request"
				operation.BadRequest = true
			}
			switch {
			case endpoint.IsExec:
				operation.Response = openAPISchema{Ref: "SuccessResponse"}
			case endpoint.DomainResponse && endpoint.Result == "many":
				operation.Response = openAPISchema{Type: "array", Items: &responseRef}
			case endpoint.DomainResponse:
				operation.Response = responseRef
				operation.NotFound = endpoint.Result == "one"
			default:
				operation.Response = openAPISchemaFromGoType(endpoint.ResponseType)
			}
			operation.Path = applicationPath(features.HTTP.BasePath, operation.Path)
			result = append(result, protectOpenAPIOperation(operation, features))
		}
		if tbl.Queries.GetByID {
			result = append(result, protectOpenAPIOperation(openAPIOperation{Method: "GET", Path: applicationPath(features.HTTP.BasePath, tbl.RouteBase+"/{id}"), Name: "Get" + tbl.GoName + "ByID", Tag: tbl.Name, Params: []endpointParam{{Name: "id", JSONName: "id", Source: "path", GoType: "uuid.UUID", Required: true}}, Response: responseRef, BadRequest: true, NotFound: true, ServerError: true}, features))
		}
		if tbl.Queries.Delete {
			result = append(result, protectOpenAPIOperation(openAPIOperation{Method: "DELETE", Path: applicationPath(features.HTTP.BasePath, tbl.RouteBase+"/{id}"), Name: "Delete" + tbl.GoName, Tag: tbl.Name, Params: []endpointParam{{Name: "id", JSONName: "id", Source: "path", GoType: "uuid.UUID", Required: true}}, Response: openAPISchema{Ref: "DeletedResponse"}, BadRequest: true, ServerError: true}, features))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Path == result[j].Path {
			return result[i].Method < result[j].Method
		}
		return result[i].Path < result[j].Path
	})
	return result
}

func protectOpenAPIOperation(operation openAPIOperation, features FeatureOptions) openAPIOperation {
	if !features.Auth.Enabled {
		return operation
	}
	key := strings.ToUpper(operation.Method) + " " + operation.Path
	policy, ok := features.Auth.Policies[key]
	if ok && policy.Public {
		return operation
	}
	if !ok && strings.EqualFold(features.Auth.DefaultPolicy, "allow") {
		return operation
	}
	if strings.EqualFold(features.Auth.Strategy, "basic") {
		operation.Security = []string{"basicAuth"}
	} else {
		operation.Security = []string{"bearerAuth"}
	}
	operation.Unauthorized = true
	if len(policy.Roles) > 0 {
		operation.Forbidden = true
	}
	return operation
}

func writeOpenAPIComponents(b *strings.Builder, tables []table, features FeatureOptions) {
	b.WriteString("components:\n")
	if features.Auth.Enabled {
		b.WriteString("  securitySchemes:\n")
		if strings.EqualFold(features.Auth.Strategy, "basic") {
			b.WriteString("    basicAuth:\n")
			b.WriteString("      type: http\n")
			b.WriteString("      scheme: basic\n")
		} else {
			b.WriteString("    bearerAuth:\n")
			b.WriteString("      type: http\n")
			b.WriteString("      scheme: bearer\n")
			b.WriteString("      bearerFormat: JWT\n")
		}
	}
	b.WriteString("  schemas:\n")
	writeOpenAPIObjectSchema(b, "ErrorResponse", []openAPIProperty{{Name: "slug", GoType: "string", Required: true}, {Name: "error", GoType: "string"}})
	writeOpenAPIObjectSchema(b, "SuccessResponse", []openAPIProperty{{Name: "ok", GoType: "bool", Required: true}})
	writeOpenAPIObjectSchema(b, "DeletedResponse", []openAPIProperty{{Name: "deleted", GoType: "bool", Required: true}})
	if features.Auth.Enabled && strings.EqualFold(features.Auth.Strategy, "jwt") {
		writeOpenAPIObjectSchema(b, "AuthRequest", []openAPIProperty{{Name: "username", GoType: "string", Required: true}, {Name: "password", GoType: "string", Required: true}})
		props := []openAPIProperty{{Name: "token", GoType: "string", Required: true}}
		if features.Auth.JWTRefreshToken {
			props = append(props, openAPIProperty{Name: "refresh_token", GoType: "string"})
		}
		writeOpenAPIObjectSchema(b, "TokenResponse", props)
	}
	if features.HTTP.Health {
		writeOpenAPIObjectSchema(b, "HealthResponse", []openAPIProperty{{Name: "status", GoType: "string", Required: true}})
	}
	for _, tbl := range tables {
		if features.Auth.Enabled && strings.EqualFold(features.Auth.Strategy, "jwt") && isAuthIdentityTable(tbl, features.Auth) {
			continue
		}
		request := make([]openAPIProperty, 0, len(tbl.CreateCols))
		for _, col := range tbl.CreateCols {
			request = append(request, openAPIProperty{Name: col.JSONName, GoType: col.GoType, Required: col.Required})
		}
		writeOpenAPIObjectSchema(b, tbl.GoName+"Request", request)
		response := make([]openAPIProperty, 0, len(tbl.Columns))
		for _, col := range tbl.Columns {
			response = append(response, openAPIProperty{Name: col.JSONName, GoType: col.GoType, Required: true})
		}
		writeOpenAPIObjectSchema(b, tbl.GoName+"Response", response)
		for _, endpoint := range tbl.Endpoints {
			if len(endpoint.BodyParams) == 0 {
				continue
			}
			props := make([]openAPIProperty, 0, len(endpoint.BodyParams))
			for _, param := range endpoint.BodyParams {
				props = append(props, openAPIProperty{Name: param.JSONName, GoType: param.GoType, Required: param.Required})
			}
			writeOpenAPIObjectSchema(b, endpoint.Name+"Request", props)
		}
	}
}

type openAPIProperty struct {
	Name     string
	GoType   string
	Required bool
}

func writeOpenAPIObjectSchema(b *strings.Builder, name string, properties []openAPIProperty) {
	fmt.Fprintf(b, "    %s:\n", name)
	b.WriteString("      type: object\n")
	var required []string
	for _, property := range properties {
		if property.Required {
			required = append(required, property.Name)
		}
	}
	if len(required) > 0 {
		b.WriteString("      required:\n")
		for _, name := range required {
			fmt.Fprintf(b, "        - %s\n", name)
		}
	}
	b.WriteString("      properties:\n")
	for _, property := range properties {
		fmt.Fprintf(b, "        %s:\n", property.Name)
		writeOpenAPITypeSchema(b, "          ", property.GoType)
	}
}

func openAPIParams(operation openAPIOperation) []endpointParam {
	params := make([]endpointParam, 0, len(operation.Params))
	seen := map[string]bool{}
	for _, param := range operation.Params {
		if param.Source != "path" && param.Source != "query" {
			continue
		}
		if param.JSONName == "" {
			param.JSONName = param.Name
		}
		params = append(params, param)
		seen[param.JSONName] = true
	}
	for _, segment := range strings.Split(operation.Path, "/") {
		if len(segment) < 3 || segment[0] != '{' || segment[len(segment)-1] != '}' {
			continue
		}
		name := segment[1 : len(segment)-1]
		if !seen[name] {
			params = append(params, endpointParam{Name: name, JSONName: name, Source: "path", GoType: "string", Required: true})
		}
	}
	return params
}

func openAPISchemaFromGoType(goType string) openAPISchema {
	goType = strings.TrimSpace(goType)
	if strings.HasPrefix(goType, "[]") {
		item := openAPISchemaFromGoType(strings.TrimPrefix(goType, "[]"))
		return openAPISchema{Type: "array", Items: &item}
	}
	schema := openAPISchema{Type: openAPIType(goType)}
	switch goType {
	case "uuid.UUID":
		schema.Format = "uuid"
	case "time.Time":
		schema.Format = "date-time"
	case "int32":
		schema.Format = "int32"
	case "int64", "int":
		schema.Format = "int64"
	case "float32":
		schema.Format = "float"
	case "float64":
		schema.Format = "double"
	}
	return schema
}

func writeOpenAPISchema(b *strings.Builder, indent string, schema openAPISchema) {
	if schema.Ref != "" {
		fmt.Fprintf(b, "%s$ref: '#/components/schemas/%s'\n", indent, schema.Ref)
		return
	}
	fmt.Fprintf(b, "%stype: %s\n", indent, schema.Type)
	if schema.Format != "" {
		fmt.Fprintf(b, "%sformat: %s\n", indent, schema.Format)
	}
	if schema.Items != nil {
		fmt.Fprintf(b, "%sitems:\n", indent)
		writeOpenAPISchema(b, indent+"  ", *schema.Items)
	}
}

func writeOpenAPITypeSchema(b *strings.Builder, indent, goType string) {
	writeOpenAPISchema(b, indent, openAPISchemaFromGoType(goType))
}

func writeOpenAPIParameterSchema(b *strings.Builder, indent string, param endpointParam) {
	schema := openAPISchemaFromGoType(param.GoType)
	if param.Type == "time" || param.Type == "null_time" {
		schema.Format = "date"
	}
	writeOpenAPISchema(b, indent, schema)
}

func openAPIType(goType string) string {
	switch strings.TrimSpace(goType) {
	case "int", "int8", "int16", "int32", "int64":
		return "integer"
	case "float32", "float64":
		return "number"
	case "bool":
		return "boolean"
	default:
		return "string"
	}
}

func openAPIHTTPPort(port int) int {
	if port == 0 {
		return 8080
	}
	return port
}

func defaultOpenAPIValue(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func applicationPath(basePath, path string) string {
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	basePath = strings.TrimSpace(basePath)
	if basePath == "" || basePath == "/" {
		return path
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	return strings.TrimRight(basePath, "/") + path
}
