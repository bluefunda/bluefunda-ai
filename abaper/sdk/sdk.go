package sdk

import (
	"fmt"
	"time"

	"github.com/bluefunda/abaper/internal/adt"
	"github.com/bluefunda/abaper/types"
)

// Defaults applied by New when the corresponding Option is left zero.
const (
	defaultClient         = "100"
	defaultLanguage       = "EN"
	defaultConnectTimeout = 30 * time.Second
	defaultRequestTimeout = 120 * time.Second
)

// Options configures a SAP ADT client. Host, Username, and Password are
// required; the rest have sensible defaults.
type Options struct {
	// Host is the SAP ADT base URL, e.g. "https://sap.example.com:44300". Required.
	Host string
	// Client is the SAP client (mandant) number. Defaults to "100".
	Client string
	// Username is the SAP user. Required.
	Username string
	// Password is the SAP password. Required.
	Password string
	// Language is the logon language. Defaults to "EN".
	Language string
	// AllowSelfSigned skips TLS certificate verification. Useful for trial
	// systems with self-signed certs; do not enable against production.
	AllowSelfSigned bool
	// ConnectTimeout bounds the initial connection. Defaults to 30s.
	ConnectTimeout time.Duration
	// RequestTimeout bounds each ADT request. Defaults to 120s.
	RequestTimeout time.Duration
	// Debug enables verbose logging in the underlying client.
	Debug bool
	// SkipConnect returns the client without calling Authenticate. The caller
	// is then responsible for authenticating before issuing requests.
	SkipConnect bool
}

// New builds a SAP ADT client from Options and, unless SkipConnect is set,
// authenticates before returning. The returned value implements the full
// types.ADTClient interface.
func New(opts Options) (types.ADTClient, error) {
	if opts.Host == "" {
		return nil, fmt.Errorf("sdk: Host is required")
	}
	if opts.Username == "" {
		return nil, fmt.Errorf("sdk: Username is required")
	}
	if opts.Password == "" {
		return nil, fmt.Errorf("sdk: Password is required")
	}

	cfg := &types.ADTConfig{
		Host:            opts.Host,
		Client:          orDefault(opts.Client, defaultClient),
		Username:        opts.Username,
		Password:        opts.Password,
		Language:        orDefault(opts.Language, defaultLanguage),
		AllowSelfSigned: opts.AllowSelfSigned,
		ConnectTimeout:  opts.ConnectTimeout,
		RequestTimeout:  opts.RequestTimeout,
		Debug:           opts.Debug,
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = defaultConnectTimeout
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = defaultRequestTimeout
	}

	client := adt.NewADTClient(cfg)

	if !opts.SkipConnect {
		if err := client.Authenticate(); err != nil {
			return nil, fmt.Errorf("sdk: authentication failed: %w", err)
		}
	}

	return client, nil
}

// NewFromConfig builds a client from a fully-specified ADTConfig without
// authenticating. Use this for advanced scenarios that need direct control
// over the configuration; most callers should prefer New.
func NewFromConfig(cfg *types.ADTConfig) types.ADTClient {
	return adt.NewADTClient(cfg)
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
