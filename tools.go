// Copyright Contributors to the Open Cluster Management project

//go:build tools
// +build tools

// This file is excluded from compilation unless the build flag -tags tools is used.
// This pattern allows us to track build tools dependencies in go.mod.
package tools

import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
)
