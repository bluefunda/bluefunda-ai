package adt

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/bluefunda/abaper/types"
)

// These tests marshal the create-shell XML payloads used by the various
// Create* methods and assert on the exact type ID / root element abap-adt-api
// (the reference implementation) expects SAP ADT to receive. They exist
// because a prior bug shipped CreateFunctionGroup with the function *module*
// type ID (FUGR/FF) and the wrong root element, which SAP silently rejected
// with "Data is invalid and could not be converted" only when tested live.

func TestFunctionGroupCreatePayload(t *testing.T) {
	payload := functionGroupCreatePayload{
		FgroupNS:    "http://www.sap.com/adt/functions/groups",
		AdtcoreNS:   "http://www.sap.com/adt/core",
		Description: "test group",
		Name:        "ZTEST_FG",
		Type:        "FUGR/F",
		Responsible: "DEVELOPER",
		PackageRef:  classPackageRef{Name: "$TMP"},
	}
	out, err := xml.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xmlStr := string(out)

	if !strings.Contains(xmlStr, `<group:abapFunctionGroup`) {
		t.Errorf("expected root element group:abapFunctionGroup, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `adtcore:type="FUGR/F"`) {
		t.Errorf("expected adtcore:type=\"FUGR/F\" (function GROUP, not module), got: %s", xmlStr)
	}
	if strings.Contains(xmlStr, `FUGR/FF`) {
		t.Errorf("function group payload must not use the function MODULE type ID FUGR/FF: %s", xmlStr)
	}
}

func TestFunctionModuleCreatePayload(t *testing.T) {
	payload := functionModuleCreatePayload{
		FmoduleNS:   "http://www.sap.com/adt/functions/fmodules",
		AdtcoreNS:   "http://www.sap.com/adt/core",
		Description: "test module",
		Name:        "ZTEST_FM",
		Type:        "FUGR/FF",
		ContainerRef: functionContainerRef{
			Name: "ZTEST_FG",
			Type: "FUGR/F",
			URI:  "/sap/bc/adt/functions/groups/ztest_fg",
		},
	}
	out, err := xml.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xmlStr := string(out)

	if !strings.Contains(xmlStr, `<fmodule:abapFunctionModule`) {
		t.Errorf("expected root element fmodule:abapFunctionModule, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `adtcore:type="FUGR/FF"`) {
		t.Errorf("expected adtcore:type=\"FUGR/FF\", got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<adtcore:containerRef`) {
		t.Errorf("function module create must reference its containing group via containerRef, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `adtcore:name="ZTEST_FG"`) {
		t.Errorf("containerRef must name the containing function group, got: %s", xmlStr)
	}
}

func TestDDLSourceCreatePayload(t *testing.T) {
	payload := ddlSourceCreatePayload{
		DdlNS:       "http://www.sap.com/adt/ddic/ddlsources",
		AdtcoreNS:   "http://www.sap.com/adt/core",
		Description: "test CDS view",
		Name:        "ZTEST_CDS",
		Type:        "DDLS/DF",
		Responsible: "DEVELOPER",
		PackageRef:  classPackageRef{Name: "$TMP"},
	}
	out, err := xml.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xmlStr := string(out)

	if !strings.Contains(xmlStr, `<ddl:ddlSource`) {
		t.Errorf("expected root element ddl:ddlSource, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `adtcore:type="DDLS/DF"`) {
		t.Errorf("expected adtcore:type=\"DDLS/DF\", got: %s", xmlStr)
	}
}

// TestSRVDSourceCreatePayload guards the srvd:srvdSourceType="S" attribute,
// which SAP requires and silently rejects with "Service Definition type ''
// does not exist" if missing — found only via live testing against
// abap.bluefunda.com (see docs/OBJECT_TYPE_PARITY_PLAN.md item 6 follow-up).
func TestSRVDSourceCreatePayload(t *testing.T) {
	payload := srvdSourceCreatePayload{
		SrvdNS:         "http://www.sap.com/adt/ddic/srvdsources",
		AdtcoreNS:      "http://www.sap.com/adt/core",
		Description:    "test service definition",
		Name:           "ZTEST_SD",
		Type:           "SRVD/SRV",
		Language:       "EN",
		MasterLanguage: "EN",
		Responsible:    "DEVELOPER",
		SourceType:     "S",
		PackageRef:     classPackageRef{Name: "$TMP"},
	}
	out, err := xml.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xmlStr := string(out)

	if !strings.Contains(xmlStr, `<srvd:srvdSource`) {
		t.Errorf("expected root element srvd:srvdSource, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `adtcore:type="SRVD/SRV"`) {
		t.Errorf("expected adtcore:type=\"SRVD/SRV\", got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `srvd:srvdSourceType="S"`) {
		t.Errorf(`expected srvd:srvdSourceType="S" (SAP rejects creation without it), got: %s`, xmlStr)
	}
}

// TestServiceBindingCreatePayload guards the srvb:serviceBinding shape SAP
// accepts for a full create-in-one-POST (no separate lock/PUT needed, unlike
// domains/data elements) — confirmed live against abap.bluefunda.com.
func TestServiceBindingCreatePayload(t *testing.T) {
	payload := serviceBindingPayload{
		SrvbNS:         "http://www.sap.com/adt/ddic/ServiceBindings",
		AdtcoreNS:      "http://www.sap.com/adt/core",
		Description:    "test service binding",
		Name:           "ZTEST_SB",
		Type:           "SRVB/SVB",
		Language:       "EN",
		MasterLanguage: "EN",
		Responsible:    "DEVELOPER",
		PackageRef:     classPackageRef{Name: "$TMP"},
		Services: srvbServices{
			Name: "ZTEST_SB",
			Content: srvbContent{
				Version:           "0001",
				ServiceDefinition: srvbServiceRef{Name: "ZTEST_SD"},
			},
		},
		Binding: srvbBinding{
			Category:       "0",
			Type:           "ODATA",
			Version:        "V4",
			Implementation: srvbImplementation{Name: ""},
		},
	}
	out, err := xml.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xmlStr := string(out)

	if !strings.Contains(xmlStr, `<srvb:serviceBinding`) {
		t.Errorf("expected root element srvb:serviceBinding, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `adtcore:type="SRVB/SVB"`) {
		t.Errorf("expected adtcore:type=\"SRVB/SVB\", got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `srvb:type="ODATA"`) || !strings.Contains(xmlStr, `srvb:version="V4"`) {
		t.Errorf("expected ODATA V4 binding type, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<srvb:serviceDefinition adtcore:name="ZTEST_SD"`) {
		t.Errorf("expected serviceDefinition reference to ZTEST_SD, got: %s", xmlStr)
	}
}

// TestBuildServiceBindingPayloadDefaults guards the V4/UI defaults applied
// when the caller doesn't specify a binding version/category — this feature
// targets RAP V4 services, so V4+UI ("0") is the sensible default.
func TestBuildServiceBindingPayloadDefaults(t *testing.T) {
	c := &ADTClientImpl{config: &types.ADTConfig{Username: "developer"}}
	out, err := c.buildServiceBindingPayload("ZTEST_SB", types.ServiceBindingProperties{
		ServiceDefinitionName: "ZTEST_SD",
	})
	if err != nil {
		t.Fatalf("buildServiceBindingPayload: %v", err)
	}
	xmlStr := string(out)
	if !strings.Contains(xmlStr, `srvb:version="V4"`) {
		t.Errorf("expected default binding version V4, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `srvb:category="0"`) {
		t.Errorf("expected default binding category \"0\" (UI), got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `adtcore:responsible="DEVELOPER"`) {
		t.Errorf("expected responsible to be uppercased from config username, got: %s", xmlStr)
	}
}

// TestServiceBindingReadResponseParsing guards parsing of the live SAP
// srvb:serviceBinding GET response shape (namespace-prefixed elements and
// attributes, matched here by local name only).
func TestServiceBindingReadResponseParsing(t *testing.T) {
	sample := `<?xml version="1.0" encoding="utf-8"?><srvb:serviceBinding srvb:contract="C2" srvb:releaseSupported="true" srvb:published="true" srvb:bindingCreated="true" adtcore:responsible="DEVELOPER" adtcore:name="ZTEST_SB" adtcore:type="SRVB/SVB" adtcore:version="active" adtcore:description="test binding" xmlns:srvb="http://www.sap.com/adt/ddic/ServiceBindings" xmlns:adtcore="http://www.sap.com/adt/core"><srvb:services srvb:name="ZTEST_SB"><srvb:content srvb:version="0001"><srvb:serviceDefinition adtcore:uri="/sap/bc/adt/ddic/srvd/sources/ztest_sd" adtcore:type="SRVD/SRV" adtcore:name="ZTEST_SD"/></srvb:content></srvb:services><srvb:binding srvb:type="ODATA" srvb:version="V4" srvb:category="0"><srvb:implementation adtcore:name="ZTEST_SB"/></srvb:binding></srvb:serviceBinding>`

	var parsed serviceBindingReadResponse
	if err := xml.Unmarshal([]byte(sample), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Description != "test binding" {
		t.Errorf("expected description %q, got %q", "test binding", parsed.Description)
	}
	if !parsed.Published {
		t.Errorf("expected published=true")
	}
	if parsed.Version != "active" {
		t.Errorf("expected version=active, got %q", parsed.Version)
	}
	if parsed.Services.Content.ServiceDefinition.Name != "ZTEST_SD" {
		t.Errorf("expected service definition name ZTEST_SD, got %q", parsed.Services.Content.ServiceDefinition.Name)
	}
	if parsed.Binding.Version != "V4" || parsed.Binding.Category != "0" {
		t.Errorf("expected binding V4/category 0, got version=%q category=%q", parsed.Binding.Version, parsed.Binding.Category)
	}
}
