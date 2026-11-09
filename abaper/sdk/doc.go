// Package sdk is the public Go SDK for driving SAP ABAP Development Tools (ADT)
// operations programmatically — the same client the abaper CLI and REST server
// use internally, exposed for embedding in your own Go programs.
//
// Create a client with New and call any method on the returned types.ADTClient:
//
//	client, err := sdk.New(sdk.Options{
//	    Host:            "https://sap.example.com:44300",
//	    Client:          "001",
//	    Username:        "developer",
//	    Password:        os.Getenv("SAP_PASSWORD"),
//	    AllowSelfSigned: true,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	src, err := client.GetProgram(context.Background(), "ZHELLO_WORLD")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(src.Source)
//
// New authenticates before returning unless Options.SkipConnect is set. The
// returned value implements the full types.ADTClient interface (read, write,
// activate, search, package browse, transports, and language features).
//
// This package supersedes the older lib package, which remains for backward
// compatibility.
package sdk
