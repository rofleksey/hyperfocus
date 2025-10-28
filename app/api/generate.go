package api

//go:generate go tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config oapi.yaml openapi.yaml
//go:generate npx @redocly/cli build-docs --title "Hyperfocus API" -o docs/index.html openapi.yaml
