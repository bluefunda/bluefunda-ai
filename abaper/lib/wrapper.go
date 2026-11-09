// Package lib provides library functions for abaper that can be imported by other Go programs
// This package serves as a thin wrapper around the internal/adt package
package lib

import (
	"fmt"
	"time"

	"github.com/bluefunda/abaper/internal/adt"
	"github.com/bluefunda/abaper/types"
)

// NewADTClient creates a new ADT client using the internal implementation
// This is the library version that external packages can import
func NewADTClient(config *types.ADTConfig) types.ADTClient {
	return adt.NewADTClient(config)
}

// CreateADTClient creates an ADT client from simplified parameters
// This provides a higher-level interface that's easier to use from other packages
func CreateADTClient(host, client, username, password string) (types.ADTClient, error) {
	if host == "" {
		return nil, fmt.Errorf("ADT host is required")
	}
	if username == "" {
		return nil, fmt.Errorf("ADT username is required")
	}
	if password == "" {
		return nil, fmt.Errorf("ADT password is required")
	}

	config := &types.ADTConfig{
		Host:            host,
		Client:          client,
		Username:        username,
		Password:        password,
		Language:        "EN",
		AllowSelfSigned: true,
		ConnectTimeout:  30 * time.Second,
		RequestTimeout:  120 * time.Second,
		Debug:           false,
	}

	// Set default client if not specified
	if config.Client == "" {
		config.Client = "100"
	}

	adtClient := NewADTClient(config)

	// Test connection
	if err := adtClient.Authenticate(); err != nil {
		return nil, fmt.Errorf("ADT authentication failed: %w", err)
	}

	return adtClient, nil
}
