// Package abaper is the root of the ABAPer module — a CLI tool and Go SDK
// for SAP ABAP development on the BlueFunda platform.
//
// # CLI
//
// The abaper binary communicates with ABAPer APIs through the KrakenD gateway
// (abaper-gw) via OAuth2 device authorization. Run it bare to open the
// interactive chat UI, or use a subcommand:
//
//	abaper                    # interactive TUI (Bubble Tea chat)
//	abaper ai chat            # AI pair-programmer chat (flag-driven)
//	abaper generate           # generate ABAP from a prompt
//	abaper deploy             # deploy objects to the SAP system
//	abaper list objects       # list ABAP objects in a package
//	abaper list packages      # list packages
//	abaper login / logout     # OAuth2 device flow
//	abaper version            # print version
//
// # Go SDK
//
// Import the SDK to drive SAP ADT operations directly from Go:
//
//	import "github.com/bluefunda/abaper/lib"
//
//	client, err := lib.CreateADTClient(host, sapClient, user, password)
//	src, err := client.GetProgram(ctx, "ZMY_PROGRAM")
//
// The SDK is structured around small capability interfaces defined in the
// [types] package:
//
//   - [types.SourceReader]    — retrieve ABAP object source
//   - [types.SourceWriter]    — create and update ABAP objects
//   - [types.PackageBrowser]  — search packages and objects
//   - [types.ObjectActivator] — activate objects and run unit tests
//   - [types.LangFeatures]    — LSP-style syntax check, completion, navigation
//
// The concrete implementation is [internal/adt.ADTClientImpl], which satisfies
// all interfaces and auto-recovers from session expiry via transparent
// re-authentication.
//
// # LSP Server
//
// An LSP server that bridges ABAP language intelligence to any LSP-capable
// editor is exposed via the [lsp] package:
//
//	import "github.com/bluefunda/abaper/lsp"
//
//	srv := lsp.NewServer(adtClient, workDir)
//	srv.RunStdio()  // or RunTCP(":2087")
//
// # REST Server
//
// A REST API server with CLI-feature parity (no AI) is in [rest/server]:
//
//	import "github.com/bluefunda/abaper/rest/server"
//
//	srv := server.NewRestServer(cfg, logger, adtClient)
//	srv.Start("8080")
package abaper
