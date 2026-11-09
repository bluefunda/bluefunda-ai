package adt

import "testing"

// sampleNodeStructureXML mirrors the application/vnd.sap.as+xml shape SAP ADT
// returns from /repository/nodestructure: an <asx:abap> envelope with repository
// objects under TREE_CONTENT and type descriptors under OBJECT_TYPES. It includes
// a virtual folder row (empty OBJECT_NAME) that must be filtered out.
const sampleNodeStructureXML = `<?xml version="1.0" encoding="utf-8"?>
<asx:abap version="1.0" xmlns:asx="http://www.sap.com/abapxml"><asx:values><DATA>
<TREE_CONTENT>
<SEU_ADT_REPOSITORY_OBJ_NODE><OBJECT_TYPE>DEVC/Q4H</OBJECT_TYPE><OBJECT_NAME/><TECH_NAME>$TMP</TECH_NAME><OBJECT_URI/><EXPANDABLE>X</EXPANDABLE><DESCRIPTION/></SEU_ADT_REPOSITORY_OBJ_NODE>
<SEU_ADT_REPOSITORY_OBJ_NODE><OBJECT_TYPE>PROG/P</OBJECT_TYPE><OBJECT_NAME>ZHELLO_WORLD</OBJECT_NAME><TECH_NAME>ZHELLO_WORLD</TECH_NAME><OBJECT_URI>/sap/bc/adt/programs/programs/zhello_world</OBJECT_URI><EXPANDABLE/><DESCRIPTION>Hello World</DESCRIPTION></SEU_ADT_REPOSITORY_OBJ_NODE>
<SEU_ADT_REPOSITORY_OBJ_NODE><OBJECT_TYPE>TABL/DT</OBJECT_TYPE><OBJECT_NAME>ZABAPGIT</OBJECT_NAME><TECH_NAME>ZABAPGIT</TECH_NAME><OBJECT_URI>/sap/bc/adt/ddic/tables/zabapgit</OBJECT_URI><EXPANDABLE>X</EXPANDABLE><DESCRIPTION>abapGit table</DESCRIPTION></SEU_ADT_REPOSITORY_OBJ_NODE>
</TREE_CONTENT>
<CATEGORIES></CATEGORIES>
<OBJECT_TYPES>
<SEU_ADT_OBJECT_TYPE_INFO><OBJECT_TYPE>PROG/P</OBJECT_TYPE><CATEGORY_TAG>source_library</CATEGORY_TAG><OBJECT_TYPE_LABEL>Program</OBJECT_TYPE_LABEL></SEU_ADT_OBJECT_TYPE_INFO>
<SEU_ADT_OBJECT_TYPE_INFO><OBJECT_TYPE>TABL/DT</OBJECT_TYPE><CATEGORY_TAG>dictionary</CATEGORY_TAG><OBJECT_TYPE_LABEL>Database Table</OBJECT_TYPE_LABEL></SEU_ADT_OBJECT_TYPE_INFO>
</OBJECT_TYPES>
</DATA></asx:values></asx:abap>`

func TestParseNodeStructureXML(t *testing.T) {
	result, err := parseNodeStructureXML([]byte(sampleNodeStructureXML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Virtual folder row (empty OBJECT_NAME) must be filtered; two real objects remain.
	if len(result.Nodes) != 2 {
		t.Fatalf("expected 2 object nodes, got %d: %+v", len(result.Nodes), result.Nodes)
	}

	prog := result.Nodes[0]
	if prog.Name != "ZHELLO_WORLD" || prog.Type != "PROG/P" {
		t.Errorf("unexpected first node: %+v", prog)
	}
	if prog.URI != "/sap/bc/adt/programs/programs/zhello_world" {
		t.Errorf("expected program URI, got %q", prog.URI)
	}
	if prog.Description != "Hello World" {
		t.Errorf("expected description, got %q", prog.Description)
	}
	if prog.Expandable {
		t.Errorf("program should not be expandable")
	}

	tbl := result.Nodes[1]
	if tbl.Name != "ZABAPGIT" || !tbl.Expandable {
		t.Errorf("expected expandable table node, got %+v", tbl)
	}

	if len(result.ObjectTypes) != 2 {
		t.Fatalf("expected 2 object types, got %d", len(result.ObjectTypes))
	}
	if result.ObjectTypes[0].Type != "PROG/P" || result.ObjectTypes[0].Label != "Program" {
		t.Errorf("unexpected object type: %+v", result.ObjectTypes[0])
	}
}

func TestParseNodeStructureXML_Invalid(t *testing.T) {
	if _, err := parseNodeStructureXML([]byte("not xml")); err == nil {
		t.Fatal("expected error for invalid XML")
	}
}
