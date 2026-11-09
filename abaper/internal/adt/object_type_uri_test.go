package adt

import "testing"

// TestObjectTypeToURI covers every alias the REST layer (rest/server/server.go)
// accepts for object_type across get/create/save handlers. A prior bug let
// "FUNCTIONGROUP" through everywhere except here, so ActivateObject and
// SyntaxCheck rejected it with "unsupported object type for activation" even
// though it read/created/saved fine.
func TestObjectTypeToURI(t *testing.T) {
	cases := []struct {
		objectType string
		name       string
		wantURI    string
	}{
		{"PROG", "zfoo", "/sap/bc/adt/programs/programs/zfoo"},
		{"PROGRAM", "zfoo", "/sap/bc/adt/programs/programs/zfoo"},
		{"CLAS", "zcl_foo", "/sap/bc/adt/oo/classes/zcl_foo"},
		{"CLASS", "zcl_foo", "/sap/bc/adt/oo/classes/zcl_foo"},
		{"INTF", "zif_foo", "/sap/bc/adt/oo/interfaces/zif_foo"},
		{"INTERFACE", "zif_foo", "/sap/bc/adt/oo/interfaces/zif_foo"},
		{"FUGR", "zfg", "/sap/bc/adt/functions/groups/zfg"},
		{"FUNCTION_GROUP", "zfg", "/sap/bc/adt/functions/groups/zfg"},
		{"FUNCTIONGROUP", "zfg", "/sap/bc/adt/functions/groups/zfg"},
		{"INCL", "zincl", "/sap/bc/adt/programs/includes/zincl"},
		{"INCLUDE", "zincl", "/sap/bc/adt/programs/includes/zincl"},
		{"DDLS", "zcds", "/sap/bc/adt/ddic/ddl/sources/zcds"},
		{"DATA_DEFINITION", "zcds", "/sap/bc/adt/ddic/ddl/sources/zcds"},
		{"TABL", "ztab", "/sap/bc/adt/ddic/tables/ztab"},
		{"TABLE", "ztab", "/sap/bc/adt/ddic/tables/ztab"},
		{"STRU", "zstru", "/sap/bc/adt/ddic/structures/zstru"},
		{"STRUCTURE", "zstru", "/sap/bc/adt/ddic/structures/zstru"},
	}
	for _, tc := range cases {
		t.Run(tc.objectType, func(t *testing.T) {
			got, err := objectTypeToURI(tc.objectType, tc.name)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantURI {
				t.Errorf("objectTypeToURI(%q, %q) = %q, want %q", tc.objectType, tc.name, got, tc.wantURI)
			}
		})
	}
}

func TestObjectTypeToURI_Unsupported(t *testing.T) {
	if _, err := objectTypeToURI("BOGUS", "x"); err == nil {
		t.Fatal("expected error for unsupported object type")
	}
}
