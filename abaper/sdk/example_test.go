package sdk_test

import (
	"fmt"

	"github.com/bluefunda/abaper/sdk"
)

// Example shows the minimal construction of an ADT client. Real use requires a
// reachable SAP system, so this example uses SkipConnect to avoid a live call.
func Example() {
	client, err := sdk.New(sdk.Options{
		Host:            "https://sap.example.com:44300",
		Client:          "001",
		Username:        "developer",
		Password:        "secret",
		AllowSelfSigned: true,
		SkipConnect:     true, // omit in real use to authenticate on New
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	_ = client
	fmt.Println("client created")
	// Output: client created
}

// Example_missingCredentials shows the validation error when a required field
// is omitted.
func Example_missingCredentials() {
	_, err := sdk.New(sdk.Options{Host: "https://sap.example.com:44300"})
	fmt.Println(err)
	// Output: sdk: Username is required
}
