module github.com/quantalogic/openai-api-simulator

go 1.22

require (
	github.com/google/uuid v1.5.0
	github.com/stretchr/testify v1.8.4
)

// Replace this upstream module path with the local repo for development so
// import paths like `github.com/openai/openai-api-simulator` will resolve
// to this local code. This keeps `quantalogic` as the canonical module
// path for commits and CI but allows local forks that still reference
// `github.com/openai/openai-api-simulator` to work without network fetch.
// Historically this project was forked from `github.com/openai/openai-api-simulator`.
// To maintain compatibility with forks and older import paths, map the old
// upstream module to the local repository root when developing locally.
//
// Note: the canonical module path for this repo is
// `github.com/quantalogic/openai-api-simulator` (see top-level `module`)
// so this replace directive can be removed for CI or if you prefer strict
// import path checks.
replace github.com/openai/openai-api-simulator => .

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
