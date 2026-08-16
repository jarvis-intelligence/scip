module github.com/scip-code/scip/swift

go 1.25.0

replace github.com/scip-code/scip/bindings/go/scip => ../bindings/go/scip

require (
	github.com/google/go-cmp v0.7.0
	github.com/scip-code/scip/bindings/go/scip v0.0.0-00010101000000-000000000000
	github.com/sourcegraph/jsonrpc2 v0.2.2
	github.com/stretchr/testify v1.11.1
	pgregory.net/rapid v1.3.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sourcegraph/beaut v0.0.0-20240611013027-627e4c25335a // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
