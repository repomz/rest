package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/repomz/rest/internal/appgen"
	"github.com/repomz/rest/internal/config"
)

func runList(args []string) error {
	if len(args) != 1 || args[0] != "endpoints" {
		return fmt.Errorf("usage: rest list endpoints")
	}
	if err := config.ValidateYAMLTree("rest_config"); err != nil {
		return err
	}
	endpoints, err := appgen.ListEndpoints("rest_config")
	if err != nil {
		return err
	}
	printEndpointList(os.Stdout, endpoints)
	return nil
}

func printEndpointList(w io.Writer, endpoints []appgen.EndpointInfo) {
	if len(endpoints) == 0 {
		fmt.Fprintln(w, "No endpoints found.")
		return
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "METHOD\tPATH\tNAME\tSOURCE\tACCESS\tROLES")
	for _, endpoint := range endpoints {
		roles := "-"
		if len(endpoint.Roles) > 0 {
			roles = strings.Join(endpoint.Roles, ",")
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\n",
			endpoint.Method, endpoint.Path, endpoint.Name, endpoint.Source, endpoint.Access, roles)
	}
	_ = table.Flush()
}
