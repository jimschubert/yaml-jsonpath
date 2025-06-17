module github.com/vmware-labs/yaml-jsonpath

go 1.23.0

toolchain go1.24.3

require (
	github.com/dprotaso/go-yit v0.0.0-20250513224043-18a80f8f6df4
	github.com/sergi/go-diff v1.4.0
	github.com/stretchr/testify v1.10.0
	go.yaml.in/yaml/v3 v3.0.3
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/dprotaso/go-yit => github.com/jimschubert/go-yit v0.0.1
