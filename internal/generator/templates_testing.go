package generator

const curlTemplate = `# Curl commands: {{ .Table.Name }}

All commands use the ` + "`BASE_URL`" + ` variable. Change the port before running if needed.

` + "```bash" + `
BASE_URL="${BASE_URL:-http://localhost:{{ httpPort .Features.Build.HTTPPort }}}"
` + "```" + `
{{ if .Table.Queries.GetAll }}
## Get all {{ .Table.Name }}

` + "```bash" + `
curl -sS -X GET "${BASE_URL}{{ routePath .Features.HTTP.BasePath .Table.RouteBase }}" | jq
` + "```" + `
{{ end }}
{{ if .Table.Queries.Create }}
## Create {{ .Table.Singular }}

` + "```bash" + `
curl -sS -X POST \
  -H "Content-Type: application/json" \
  -d '{{ createJSONBody .Table.CreateCols }}' \
  "${BASE_URL}{{ routePath .Features.HTTP.BasePath .Table.RouteBase }}" | jq
` + "```" + `
{{ end }}
{{ range .Table.Endpoints }}
## {{ .Name }}

` + "```bash" + `
curl -sS -X {{ .Method }}{{ if endpointNeedsBody . }} \
  -H "Content-Type: application/json" \
  -d '{{ endpointJSONBody . }}'{{ end }} \
  "${BASE_URL}{{ routePath $.Features.HTTP.BasePath (testURL .) }}" | jq
` + "```" + `
{{ end }}
{{ if .Table.Queries.GetByID }}
## Get {{ .Table.Singular }} by ID

` + "```bash" + `
curl -sS -X GET \
  "${BASE_URL}{{ routePath .Features.HTTP.BasePath .Table.RouteBase }}/00000000-0000-0000-0000-000000000001" | jq
` + "```" + `
{{ end }}
{{ if .Table.Queries.Delete }}
## Delete {{ .Table.Singular }} by ID

This command changes data.

` + "```bash" + `
curl -sS -X DELETE \
  "${BASE_URL}{{ routePath .Features.HTTP.BasePath .Table.RouteBase }}/00000000-0000-0000-0000-000000000001" | jq
` + "```" + `
{{ end }}
{{ if .Table.Queries.DeleteAll }}
## Delete all {{ .Table.Name }}

Warning: this command deletes the whole collection.

` + "```bash" + `
curl -sS -X DELETE "${BASE_URL}{{ routePath .Features.HTTP.BasePath .Table.RouteBase }}" | jq
` + "```" + `
{{ end }}`
