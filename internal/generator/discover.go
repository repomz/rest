package generator

import (
	"fmt"
	"path/filepath"
	"strings"
)

type EndpointSpec struct {
	Name   string
	Method string
	Path   string
}

func DiscoverEndpoints(sqlcPath string) ([]EndpointSpec, error) {
	cfg, err := readSQLCConfig(sqlcPath)
	if err != nil {
		return nil, err
	}
	tables, err := readSchemaTables(cfg.SchemaDirs)
	if err != nil {
		return nil, err
	}
	if len(tables) == 0 {
		return nil, fmt.Errorf("no CREATE TABLE statements found in %s", strings.Join(cfg.SchemaDirs, ", "))
	}
	queryMeta, err := readQuerierMeta(filepath.Join(cfg.DBOut, "querier.go"))
	if err != nil {
		return nil, err
	}
	paramStructs, err := readSQLCParamStructs(cfg.DBOut)
	if err != nil {
		return nil, err
	}
	optionalQueryParams, err := readSQLCOptionalQueryParams(cfg.QueriesDirs)
	if err != nil {
		return nil, err
	}
	attachEndpoints(tables, autoEndpoints(tables, queryMeta, paramStructs, optionalQueryParams))
	for i := range tables {
		tables[i].Queries = detectQueries(queryMeta, tables[i])
	}
	var result []EndpointSpec
	for _, table := range tables {
		if table.Queries.GetAll {
			result = append(result, EndpointSpec{Name: "GetAll" + table.GoPlural, Method: "GET", Path: table.RouteBase})
		}
		if table.Queries.Create {
			result = append(result, EndpointSpec{Name: "Create" + table.GoName, Method: "POST", Path: table.RouteBase})
		}
		if table.Queries.DeleteAll {
			result = append(result, EndpointSpec{Name: "DeleteAll" + table.GoPlural, Method: "DELETE", Path: table.RouteBase})
		}
		for _, endpoint := range table.Endpoints {
			result = append(result, EndpointSpec{Name: endpoint.Name, Method: endpoint.Method, Path: endpoint.Path})
		}
		if table.Queries.GetByID {
			result = append(result, EndpointSpec{Name: "Get" + table.GoName + "ByID", Method: "GET", Path: table.RouteBase + "/{id}"})
		}
		if table.Queries.Delete {
			result = append(result, EndpointSpec{Name: "Delete" + table.GoName, Method: "DELETE", Path: table.RouteBase + "/{id}"})
		}
	}
	return result, nil
}
