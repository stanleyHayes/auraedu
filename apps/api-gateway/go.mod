module github.com/auraedu/api-gateway

go 1.26.5

require (
	github.com/auraedu/platform v0.0.0
	github.com/redis/go-redis/v9 v9.21.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// Resolved locally via the workspace (go.work).
replace github.com/auraedu/platform => ../../platform
