module github.com/quantalogic/openai-api-simulator

go 1.22

require (
	github.com/google/uuid v1.5.0
	github.com/schollz/progressbar/v3 v3.14.2
	github.com/stretchr/testify v1.8.4
)

// Replace this upstream module path with the local repo for development so
// import paths like `github.com/openai/openai-api-simulator` will resolve
// to this local code. This keeps `quantalogic` as the canonical module
// path for commits and CI but allows local forks that still reference
// `github.com/openai/openai-api-simulator` to work without network fetch.
replace github.com/openai/openai-api-simulator => .

// The canonical module path `github.com/quantalogic/openai-api-simulator` must
// also be replaced locally to prevent Go from querying the proxy during
// `go mod tidy`. With GOPRIVATE set to github.com/quantalogic in the Makefile,
// Go will skip proxy lookups and use the local directory instead.
replace github.com/quantalogic/openai-api-simulator => .

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	golang.org/x/sys v0.17.0 // indirect
	golang.org/x/term v0.17.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
