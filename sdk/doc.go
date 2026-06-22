// Package sdk provides a Go client for driving bai programmatically.
//
// The client launches bai as a subprocess using --print --output-format stream-json,
// then streams typed events back to the caller. This enables IDE extensions, CI
// pipelines, and other integrations to embed bai without importing internal packages.
package sdk
