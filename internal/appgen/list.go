package appgen

import (
	"sort"
	"strings"

	"github.com/repomz/rest/internal/config"
	"github.com/repomz/rest/internal/generator"
)

type EndpointInfo struct {
	Name   string
	Method string
	Path   string
	Source string
	Access string
	Roles  []string
}

func ListEndpoints(configDir string) ([]EndpointInfo, error) {
	bundle, err := config.Load(configDir)
	if err != nil {
		return nil, err
	}
	if err := validateConfig(bundle); err != nil {
		return nil, err
	}
	ctx := NewContext(bundle)
	if err := validateReferencedYAMLInputs(ctx); err != nil {
		return nil, err
	}

	var endpoints []EndpointInfo
	add := func(endpoint config.GeneratedEndpoint, source string) {
		info := EndpointInfo{
			Name:   endpoint.Name,
			Method: strings.ToUpper(endpoint.Method),
			Path:   endpoint.Path,
			Source: source,
		}
		applyEndpointAccess(&info, bundle, endpoint.Public)
		endpoints = append(endpoints, info)
	}

	add(config.GeneratedEndpoint{Name: "Root", Method: "GET", Path: routePath(ctx.Config.Rest.HTTP.BasePath, "/"), Public: true}, "system")
	if ctx.Config.Rest.HTTP.Health.Enabled.Bool() {
		add(config.GeneratedEndpoint{Name: "Health", Method: "GET", Path: routePath(ctx.Config.Rest.HTTP.BasePath, ctx.Config.Rest.HTTP.Health.Path), Public: true}, "system")
	}
	if ctx.Config.Rest.HTTP.Readiness.Enabled.Bool() {
		add(config.GeneratedEndpoint{Name: "Readiness", Method: "GET", Path: routePath(ctx.Config.Rest.HTTP.BasePath, ctx.Config.Rest.HTTP.Readiness.Path), Public: true}, "system")
	}
	if ctx.Config.Rest.Observability.Metrics.Enabled.Bool() {
		add(config.GeneratedEndpoint{Name: "Metrics", Method: "GET", Path: routePath(ctx.Config.Rest.HTTP.BasePath, ctx.Config.Rest.Observability.Metrics.Path), Public: true}, "system")
	}
	if ctx.Config.Rest.OpenAPI.Enabled.Bool() {
		add(config.GeneratedEndpoint{Name: "OpenAPISpec", Method: "GET", Path: routePath(ctx.Config.Rest.HTTP.BasePath, ctx.Config.Rest.OpenAPI.SpecPath), Public: true}, "system")
	}
	if ctx.Config.Rest.OpenAPI.Enabled.Bool() && ctx.Config.Rest.OpenAPI.WithUI.Bool() {
		add(config.GeneratedEndpoint{Name: "OpenAPIUI", Method: "GET", Path: routePath(ctx.Config.Rest.HTTP.BasePath, ctx.Config.Rest.OpenAPI.UIPath), Public: true}, "system")
	}

	if (SQLFeature{}).Enabled(ctx) {
		sqlcPath := resolveSQLCPath(ctx.ConfigDir, ctx.Config.SQL.SQLC.Path)
		discovered, err := generator.DiscoverEndpoints(sqlcPath)
		if err != nil {
			return nil, err
		}
		for _, endpoint := range discovered {
			if authStrategy(ctx) == "jwt" && isAuthIdentityEndpoint(ctx, endpoint.Path) {
				continue
			}
			add(config.GeneratedEndpoint{
				Name:   endpoint.Name,
				Method: endpoint.Method,
				Path:   routePath(ctx.Config.Rest.HTTP.BasePath, endpoint.Path),
			}, "sqlc")
		}
	}

	if (MongoFeature{}).Enabled(ctx) {
		discovered, err := discoverMongoAuthEndpoints(ctx)
		if err != nil {
			return nil, err
		}
		for _, endpoint := range discovered {
			endpoint.Path = routePath(ctx.Config.Rest.HTTP.BasePath, endpoint.Path)
			add(endpoint, "mongo")
		}
	}

	if (SQLFeature{}).Enabled(ctx) && ctx.Config.Rest.Auth.Bool() && authStrategy(ctx) == "jwt" {
		add(config.GeneratedEndpoint{Name: "SignUp", Method: "POST", Path: routePath(ctx.Config.Rest.HTTP.BasePath, "/signup"), Public: true}, "auth")
		add(config.GeneratedEndpoint{Name: "SignIn", Method: "POST", Path: routePath(ctx.Config.Rest.HTTP.BasePath, "/signin"), Public: true}, "auth")
	}

	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Path == endpoints[j].Path {
			return endpoints[i].Method < endpoints[j].Method
		}
		return endpoints[i].Path < endpoints[j].Path
	})
	return endpoints, nil
}

func applyEndpointAccess(info *EndpointInfo, bundle config.Bundle, defaultPublic bool) {
	if !bundle.Rest.Auth.Bool() {
		info.Access = "public"
		return
	}
	if bundle.Auth == nil {
		if defaultPublic {
			info.Access = "public"
		} else {
			info.Access = "pending"
		}
		return
	}
	for _, endpoint := range bundle.Auth.Endpoints {
		if strings.EqualFold(endpoint.Method, info.Method) && endpoint.Path == info.Path {
			if endpoint.Public || !endpoint.RequireAuth {
				info.Access = "public"
				return
			}
			info.Access = "auth"
			info.Roles = append([]string(nil), endpoint.Roles...)
			sort.Strings(info.Roles)
			return
		}
	}
	if defaultPublic {
		info.Access = "public"
		return
	}
	switch bundle.Auth.Authorization.DefaultPolicy {
	case "allow":
		info.Access = "auth"
	default:
		info.Access = "denied"
	}
}
