package generator

const curlTemplate = `# Curl commands: {{ .Table.Name }}

Все команды используют переменную ` + "`BASE_URL`" + `. При необходимости измените порт перед запуском.

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

Команда изменяет данные.

` + "```bash" + `
curl -sS -X DELETE \
  "${BASE_URL}{{ routePath .Features.HTTP.BasePath .Table.RouteBase }}/00000000-0000-0000-0000-000000000001" | jq
` + "```" + `
{{ end }}
{{ if .Table.Queries.DeleteAll }}
## Delete all {{ .Table.Name }}

Внимание: команда удаляет всю коллекцию.

` + "```bash" + `
curl -sS -X DELETE "${BASE_URL}{{ routePath .Features.HTTP.BasePath .Table.RouteBase }}" | jq
` + "```" + `
{{ end }}`
