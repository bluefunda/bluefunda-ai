package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bluefunda/abaper/types"
	"go.uber.org/zap"
)

func newTestServer(t *testing.T, client types.ADTClient) *RestServer {
	t.Helper()
	if client == nil {
		return NewRestServer(&Config{}, zap.NewNop(), nil)
	}
	return NewRestServer(&Config{}, zap.NewNop(), client)
}

func doJSON(t *testing.T, rs *RestServer, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	rec := httptest.NewRecorder()
	rs.Handler().ServeHTTP(rec, req)
	return rec
}

func decodeSuccess[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var resp struct {
		Success bool   `json:"success"`
		Data    T      `json:"data"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, rec.Body.String())
	}
	if !resp.Success {
		t.Fatalf("expected success response, got error: %s", resp.Error)
	}
	return resp.Data
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var resp struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, rec.Body.String())
	}
	if resp.Success {
		t.Fatalf("expected error response, got success")
	}
	return resp.Error
}

// --- objects/get ---

func TestGetObjectHandler(t *testing.T) {
	t.Run("program", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/get", map[string]string{
			"object_type": "program",
			"object_name": "zhello_world",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeSuccess[types.ADTSourceCode](t, rec)
		if data.ObjectName != "ZHELLO_WORLD" {
			t.Errorf("expected uppercased object name, got %q", data.ObjectName)
		}
	})

	t.Run("class", func(t *testing.T) {
		var gotName string
		fake := &fakeADTClient{
			getClassFn: func(ctx context.Context, name string) (*types.ADTSourceCode, error) {
				gotName = name
				return &types.ADTSourceCode{ObjectName: name, ObjectType: "CLASS"}, nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/get", map[string]string{
			"object_type": "class",
			"object_name": "zcl_demo",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if gotName != "ZCL_DEMO" {
			t.Errorf("expected uppercased object name, got %q", gotName)
		}
	})

	t.Run("include", func(t *testing.T) {
		fake := &fakeADTClient{
			getIncludeFn: func(ctx context.Context, name string) (*types.ADTSourceCode, error) {
				return &types.ADTSourceCode{ObjectName: name, ObjectType: "INCLUDE"}, nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/get", map[string]string{
			"object_type": "include",
			"object_name": "zincl1",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeSuccess[types.ADTSourceCode](t, rec)
		if data.ObjectType != "INCLUDE" {
			t.Errorf("expected ObjectType INCLUDE, got %q", data.ObjectType)
		}
	})

	t.Run("interface", func(t *testing.T) {
		fake := &fakeADTClient{
			getInterfaceFn: func(ctx context.Context, name string) (*types.ADTSourceCode, error) {
				return &types.ADTSourceCode{ObjectName: name, ObjectType: "INTERFACE"}, nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/get", map[string]string{
			"object_type": "interface",
			"object_name": "zif_demo",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeSuccess[types.ADTSourceCode](t, rec)
		if data.ObjectType != "INTERFACE" {
			t.Errorf("expected ObjectType INTERFACE, got %q", data.ObjectType)
		}
	})

	t.Run("structure", func(t *testing.T) {
		fake := &fakeADTClient{
			getStructureFn: func(ctx context.Context, name string) (*types.ADTSourceCode, error) {
				return &types.ADTSourceCode{ObjectName: name, ObjectType: "STRUCTURE"}, nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/get", map[string]string{
			"object_type": "structure",
			"object_name": "zabpbs1",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeSuccess[types.ADTSourceCode](t, rec)
		if data.ObjectType != "STRUCTURE" {
			t.Errorf("expected ObjectType STRUCTURE, got %q", data.ObjectType)
		}
	})

	t.Run("table", func(t *testing.T) {
		fake := &fakeADTClient{
			getTableFn: func(ctx context.Context, name string) (*types.ADTSourceCode, error) {
				return &types.ADTSourceCode{ObjectName: name, ObjectType: "TABLE"}, nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/get", map[string]string{
			"object_type": "table",
			"object_name": "zabpbt1",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeSuccess[types.ADTSourceCode](t, rec)
		if data.ObjectType != "TABLE" {
			t.Errorf("expected ObjectType TABLE, got %q", data.ObjectType)
		}
	})

	t.Run("functiongroup", func(t *testing.T) {
		fake := &fakeADTClient{
			getFunctionGroupFn: func(ctx context.Context, name string) (*types.ADTSourceCode, error) {
				return &types.ADTSourceCode{ObjectName: name, ObjectType: "FUNCTIONGROUP"}, nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/get", map[string]string{
			"object_type": "functiongroup",
			"object_name": "zabpb_fg1",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeSuccess[types.ADTSourceCode](t, rec)
		if data.ObjectType != "FUNCTIONGROUP" {
			t.Errorf("expected ObjectType FUNCTIONGROUP, got %q", data.ObjectType)
		}
	})

	t.Run("function", func(t *testing.T) {
		var gotName, gotGroup string
		fake := &fakeADTClient{
			getFunctionFn: func(ctx context.Context, functionName, functionGroup string) (*types.ADTSourceCode, error) {
				gotName, gotGroup = functionName, functionGroup
				return &types.ADTSourceCode{ObjectName: functionName, ObjectType: "FUNCTION"}, nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/get", map[string]any{
			"object_type": "function",
			"object_name": "zabpb_fm1",
			"args":        []string{"zabpb_fg1"},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if gotName != "ZABPB_FM1" || gotGroup != "ZABPB_FG1" {
			t.Errorf("expected name=ZABPB_FM1 group=ZABPB_FG1, got name=%q group=%q", gotName, gotGroup)
		}
	})

	t.Run("function requires group in args", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/get", map[string]string{
			"object_type": "function",
			"object_name": "zabpb_fm1",
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("package via get route", func(t *testing.T) {
		fake := &fakeADTClient{
			getPackageContentsFn: func(ctx context.Context, name string) (*types.ADTPackage, error) {
				return &types.ADTPackage{Name: name, Objects: []types.ADTObject{{Name: "ZFOO", Type: "PROG/P"}}}, nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/get", map[string]string{
			"object_type": "package",
			"object_name": "$tmp",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeSuccess[types.ADTPackage](t, rec)
		if data.Name != "$TMP" {
			t.Errorf("expected uppercased package name, got %q", data.Name)
		}
		if len(data.Objects) != 1 || data.Objects[0].Name != "ZFOO" {
			t.Errorf("expected one object ZFOO, got %+v", data.Objects)
		}
	})

	t.Run("domain", func(t *testing.T) {
		var gotType string
		fake := &fakeADTClient{
			getTypeInfoFn: func(ctx context.Context, typeName string) (*types.ADTTypeInfo, error) {
				gotType = typeName
				return &types.ADTTypeInfo{TypeName: typeName, TypeKind: "DOMAIN"}, nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/get", map[string]string{
			"object_type": "domain",
			"object_name": "zwels",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if gotType != "ZWELS" {
			t.Errorf("expected uppercased type name passed through, got %q", gotType)
		}
		data := decodeSuccess[types.ADTTypeInfo](t, rec)
		if data.TypeKind != "DOMAIN" {
			t.Errorf("expected TypeKind DOMAIN, got %q", data.TypeKind)
		}
	})

	t.Run("data element", func(t *testing.T) {
		fake := &fakeADTClient{
			getTypeInfoFn: func(ctx context.Context, typeName string) (*types.ADTTypeInfo, error) {
				return &types.ADTTypeInfo{TypeName: typeName, TypeKind: "DATA_ELEMENT"}, nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/get", map[string]string{
			"object_type": "data_element",
			"object_name": "zdtel_tims",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeSuccess[types.ADTTypeInfo](t, rec)
		if data.TypeKind != "DATA_ELEMENT" {
			t.Errorf("expected TypeKind DATA_ELEMENT, got %q", data.TypeKind)
		}
	})

	t.Run("srvd", func(t *testing.T) {
		var gotName string
		fake := &fakeADTClient{
			getSRVDSourceFn: func(ctx context.Context, name string) (*types.ADTSourceCode, error) {
				gotName = name
				return &types.ADTSourceCode{ObjectName: name, ObjectType: "SRVD"}, nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/get", map[string]string{
			"object_type": "srvd",
			"object_name": "zabpb_sd1",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if gotName != "ZABPB_SD1" {
			t.Errorf("expected uppercased object name passed through, got %q", gotName)
		}
		data := decodeSuccess[types.ADTSourceCode](t, rec)
		if data.ObjectType != "SRVD" {
			t.Errorf("expected ObjectType SRVD, got %q", data.ObjectType)
		}
	})

	t.Run("srvb", func(t *testing.T) {
		fake := &fakeADTClient{
			getServiceBindingFn: func(ctx context.Context, name string) (*types.ADTServiceBinding, error) {
				return &types.ADTServiceBinding{Name: name, ServiceDefinitionName: "ZABPB_SD1", BindingVersion: "V4", Published: true}, nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/get", map[string]string{
			"object_type": "srvb",
			"object_name": "zabpb_sb1",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeSuccess[types.ADTServiceBinding](t, rec)
		if data.ServiceDefinitionName != "ZABPB_SD1" || data.BindingVersion != "V4" || !data.Published {
			t.Errorf("unexpected service binding data: %+v", data)
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/get", map[string]string{"object_type": "program"})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("unsupported type", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/get", map[string]string{
			"object_type": "bogus",
			"object_name": "x",
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		if got := decodeError(t, rec); got != "unsupported object type: BOGUS" {
			t.Errorf("unexpected error message: %q", got)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodGet, "/api/v1/objects/get", nil)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d", rec.Code)
		}
	})

	t.Run("no client configured", func(t *testing.T) {
		rs := newTestServer(t, nil)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/get", map[string]string{
			"object_type": "program",
			"object_name": "x",
		})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

// --- objects/create ---

func TestCreateObjectHandler(t *testing.T) {
	t.Run("program with default package", func(t *testing.T) {
		var gotPackage string
		fake := &fakeADTClient{
			createProgramFn: func(ctx context.Context, name, description, packageName, source string) error {
				gotPackage = packageName
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/create", map[string]string{
			"object_type": "program",
			"object_name": "zfoo",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if gotPackage != "$TMP" {
			t.Errorf("expected default package $TMP, got %q", gotPackage)
		}
	})

	t.Run("class", func(t *testing.T) {
		var gotDescription string
		fake := &fakeADTClient{
			createClassFn: func(ctx context.Context, name, description, packageName, source string) error {
				gotDescription = description
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/create", map[string]string{
			"object_type": "class",
			"object_name": "zcl_demo",
			"description": "Demo class",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if gotDescription != "Demo class" {
			t.Errorf("expected description to be passed through, got %q", gotDescription)
		}
	})

	t.Run("include", func(t *testing.T) {
		var called bool
		fake := &fakeADTClient{
			createIncludeFn: func(ctx context.Context, name, description, source string) error {
				called = true
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/create", map[string]string{
			"object_type": "include",
			"object_name": "zincl1",
			"description": "Demo include",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !called {
			t.Error("expected CreateInclude to be called")
		}
	})

	t.Run("interface", func(t *testing.T) {
		var called bool
		fake := &fakeADTClient{
			createInterfaceFn: func(ctx context.Context, name, description, source string) error {
				called = true
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/create", map[string]string{
			"object_type": "interface",
			"object_name": "zif_demo",
			"description": "Demo interface",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !called {
			t.Error("expected CreateInterface to be called")
		}
	})

	t.Run("structure", func(t *testing.T) {
		var called bool
		fake := &fakeADTClient{
			createStructureFn: func(ctx context.Context, name, description, source string) error {
				called = true
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/create", map[string]string{
			"object_type": "structure",
			"object_name": "zabpbs1",
			"description": "Demo structure",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !called {
			t.Error("expected CreateStructure to be called")
		}
	})

	t.Run("table", func(t *testing.T) {
		var called bool
		fake := &fakeADTClient{
			createTableFn: func(ctx context.Context, name, description, source string) error {
				called = true
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/create", map[string]string{
			"object_type": "table",
			"object_name": "zabpbt1",
			"description": "Demo table",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !called {
			t.Error("expected CreateTable to be called")
		}
	})

	t.Run("unsupported type", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/create", map[string]string{
			"object_type": "bogus",
			"object_name": "x",
			"description": "force create path, not save-mode",
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("function module requires group in args", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/create", map[string]string{
			"object_type": "function",
			"object_name": "zabpb_fm1",
			"description": "test FM",
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("function module create with group", func(t *testing.T) {
		var gotName, gotGroup string
		fake := &fakeADTClient{
			createFunctionFn: func(ctx context.Context, name, functionGroup, description, source string) error {
				gotName, gotGroup = name, functionGroup
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/create", map[string]any{
			"object_type": "function",
			"object_name": "zabpb_fm1",
			"description": "test FM",
			"args":        []string{"zabpb_fg1"},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if gotName != "ZABPB_FM1" || gotGroup != "ZABPB_FG1" {
			t.Errorf("expected name=ZABPB_FM1 group=ZABPB_FG1, got name=%q group=%q", gotName, gotGroup)
		}
	})

	t.Run("function group create", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/create", map[string]string{
			"object_type": "functiongroup",
			"object_name": "zabpb_fg1",
			"description": "test FG",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("ddls create", func(t *testing.T) {
		var gotName, gotSource string
		fake := &fakeADTClient{
			createDDLSFn: func(ctx context.Context, name, description, source string) error {
				gotName, gotSource = name, source
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/create", map[string]string{
			"object_type": "ddls",
			"object_name": "zabpb_cds1",
			"description": "test CDS",
			"source":      "define view ZABPB_CDS1 as select from t000 { key mandt }",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if gotName != "ZABPB_CDS1" || gotSource == "" {
			t.Errorf("expected name=ZABPB_CDS1 with source, got name=%q source=%q", gotName, gotSource)
		}
	})

	t.Run("srvd create", func(t *testing.T) {
		var gotName, gotSource string
		fake := &fakeADTClient{
			createSRVDFn: func(ctx context.Context, name, description, source string) error {
				gotName, gotSource = name, source
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/create", map[string]string{
			"object_type": "srvd",
			"object_name": "zabpb_sd1",
			"description": "test service definition",
			"source":      "define service ZABPB_SD1 { expose zabpb_cds1; }",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if gotName != "ZABPB_SD1" || gotSource == "" {
			t.Errorf("expected name=ZABPB_SD1 with source, got name=%q source=%q", gotName, gotSource)
		}
	})

	t.Run("srvb create requires service_binding_properties", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/create", map[string]any{
			"object_type": "srvb",
			"object_name": "zabpb_sb1",
			"description": "test service binding",
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("srvb create", func(t *testing.T) {
		var gotName string
		var gotProps types.ServiceBindingProperties
		fake := &fakeADTClient{
			createServiceBindingFn: func(ctx context.Context, name string, props types.ServiceBindingProperties) error {
				gotName, gotProps = name, props
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/create", map[string]any{
			"object_type": "srvb",
			"object_name": "zabpb_sb1",
			"description": "test service binding",
			"service_binding_properties": map[string]any{
				"service_definition_name": "zabpb_sd1",
				"binding_version":         "V4",
				"binding_category":        "0",
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if gotName != "ZABPB_SB1" {
			t.Errorf("expected uppercased name ZABPB_SB1, got %q", gotName)
		}
		if gotProps.ServiceDefinitionName != "zabpb_sd1" || gotProps.BindingVersion != "V4" {
			t.Errorf("expected properties passed through, got %+v", gotProps)
		}
		if gotProps.Description != "test service binding" {
			t.Errorf("expected description to default from request description, got %q", gotProps.Description)
		}
	})
}

// --- objects/save ---

func TestSaveObjectHandler(t *testing.T) {
	t.Run("updates class", func(t *testing.T) {
		var gotSource string
		fake := &fakeADTClient{
			updateClassFn: func(ctx context.Context, name, source string) error {
				gotSource = source
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/save", map[string]string{
			"object_type": "class",
			"object_name": "zcl_demo",
			"source":      "CLASS zcl_demo DEFINITION.",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if gotSource != "CLASS zcl_demo DEFINITION." {
			t.Errorf("unexpected source passed through: %q", gotSource)
		}
	})

	t.Run("updates program", func(t *testing.T) {
		var gotName string
		fake := &fakeADTClient{
			updateProgramFn: func(ctx context.Context, name, source string) error {
				gotName = name
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/save", map[string]string{
			"object_type": "program",
			"object_name": "zfoo",
			"source":      "REPORT zfoo.",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if gotName != "ZFOO" {
			t.Errorf("expected uppercased name, got %q", gotName)
		}
	})

	t.Run("updates include", func(t *testing.T) {
		var called bool
		fake := &fakeADTClient{
			updateIncludeFn: func(ctx context.Context, name, source string) error {
				called = true
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/save", map[string]string{
			"object_type": "include",
			"object_name": "zincl1",
			"source":      "* include",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !called {
			t.Error("expected UpdateInclude to be called")
		}
	})

	t.Run("updates interface", func(t *testing.T) {
		var called bool
		fake := &fakeADTClient{
			updateInterfaceFn: func(ctx context.Context, name, source string) error {
				called = true
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/save", map[string]string{
			"object_type": "interface",
			"object_name": "zif_demo",
			"source":      "INTERFACE zif_demo PUBLIC.\nENDINTERFACE.",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !called {
			t.Error("expected UpdateInterface to be called")
		}
	})

	t.Run("updates function group", func(t *testing.T) {
		var called bool
		fake := &fakeADTClient{
			updateFunctionGroupFn: func(ctx context.Context, name, source string) error {
				called = true
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/save", map[string]string{
			"object_type": "functiongroup",
			"object_name": "zabpb_fg1",
			"source":      "FUNCTION-POOL zabpb_fg1.",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !called {
			t.Error("expected UpdateFunctionGroup to be called")
		}
	})

	t.Run("updates domain via domain_properties", func(t *testing.T) {
		var gotName string
		fake := &fakeADTClient{
			updateDomainFn: func(ctx context.Context, name string, props types.DomainProperties) error {
				gotName = name
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/save", map[string]any{
			"object_type": "domain",
			"object_name": "zwels",
			"domain_properties": map[string]any{
				"data_type": "CHAR",
				"length":    10,
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if gotName != "ZWELS" {
			t.Errorf("expected uppercased name, got %q", gotName)
		}
	})

	t.Run("domain save requires domain_properties", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/save", map[string]string{
			"object_type": "domain",
			"object_name": "zwels",
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("updates data element via data_element_properties", func(t *testing.T) {
		var called bool
		fake := &fakeADTClient{
			updateDataElementFn: func(ctx context.Context, name string, props types.DataElementProperties) error {
				called = true
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/save", map[string]any{
			"object_type": "data_element",
			"object_name": "zdtel_tims",
			"data_element_properties": map[string]any{
				"domain_name": "ZWELS",
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !called {
			t.Error("expected UpdateDataElement to be called")
		}
	})

	t.Run("data element save requires data_element_properties", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/save", map[string]string{
			"object_type": "data_element",
			"object_name": "zdtel_tims",
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("missing source", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/save", map[string]string{
			"object_type": "class",
			"object_name": "zcl_demo",
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("updates table", func(t *testing.T) {
		var gotName string
		fake := &fakeADTClient{
			updateTableFn: func(ctx context.Context, name, source string) error {
				gotName = name
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/save", map[string]string{
			"object_type": "table",
			"object_name": "zabpbt1",
			"source":      "define table zabpbt1 { key client : mandt not null; }",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if gotName != "ZABPBT1" {
			t.Errorf("expected uppercased name, got %q", gotName)
		}
	})

	t.Run("updates structure", func(t *testing.T) {
		var called bool
		fake := &fakeADTClient{
			updateStructureFn: func(ctx context.Context, name, source string) error {
				called = true
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/save", map[string]string{
			"object_type": "structure",
			"object_name": "zabpbs1",
			"source":      "define structure zabpbs1 { id : abap.char(10); }",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !called {
			t.Error("expected UpdateStructure to be called")
		}
	})

	t.Run("updates ddls", func(t *testing.T) {
		var called bool
		fake := &fakeADTClient{
			updateDDLSFn: func(ctx context.Context, name, source string) error {
				called = true
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/save", map[string]string{
			"object_type": "ddls",
			"object_name": "zabpb_cds1",
			"source":      "define view ZABPB_CDS1 as select from t000 { key mandt }",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !called {
			t.Error("expected UpdateDDLS to be called")
		}
	})

	t.Run("updates srvd", func(t *testing.T) {
		var called bool
		fake := &fakeADTClient{
			updateSRVDFn: func(ctx context.Context, name, source string) error {
				called = true
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/save", map[string]string{
			"object_type": "srvd",
			"object_name": "zabpb_sd1",
			"source":      "define service ZABPB_SD1 { expose zabpb_cds1; }",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !called {
			t.Error("expected UpdateSRVD to be called")
		}
	})

	t.Run("updates srvb via service_binding_properties", func(t *testing.T) {
		var called bool
		fake := &fakeADTClient{
			updateServiceBindingFn: func(ctx context.Context, name string, props types.ServiceBindingProperties) error {
				called = true
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/save", map[string]any{
			"object_type": "srvb",
			"object_name": "zabpb_sb1",
			"service_binding_properties": map[string]any{
				"service_definition_name": "zabpb_sd1",
				"binding_version":         "V4",
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !called {
			t.Error("expected UpdateServiceBinding to be called")
		}
	})

	t.Run("srvb save requires service_binding_properties", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/save", map[string]string{
			"object_type": "srvb",
			"object_name": "zabpb_sb1",
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("updates function module", func(t *testing.T) {
		var gotName, gotGroup string
		fake := &fakeADTClient{
			updateFunctionFn: func(ctx context.Context, functionName, functionGroup, source string) error {
				gotName, gotGroup = functionName, functionGroup
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/save", map[string]any{
			"object_type": "function",
			"object_name": "zabpb_fm1",
			"source":      "FUNCTION zabpb_fm1.\nENDFUNCTION.",
			"args":        []string{"zabpb_fg1"},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if gotName != "ZABPB_FM1" || gotGroup != "ZABPB_FG1" {
			t.Errorf("expected name=ZABPB_FM1 group=ZABPB_FG1, got name=%q group=%q", gotName, gotGroup)
		}
	})

	t.Run("function module requires group in args", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/save", map[string]string{
			"object_type": "function",
			"object_name": "zabpb_fm1",
			"source":      "FUNCTION zabpb_fm1.\nENDFUNCTION.",
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

// --- objects/search ---

func TestSearchObjectsHandler(t *testing.T) {
	t.Run("returns matches", func(t *testing.T) {
		fake := &fakeADTClient{
			searchObjectsFn: func(ctx context.Context, pattern string, objectTypes []string) (*types.ADTSearchResult, error) {
				if pattern != "ZHELL*" {
					t.Errorf("unexpected pattern: %q", pattern)
				}
				return &types.ADTSearchResult{Objects: []types.ADTObject{{Name: "ZHELLO_WORLD", Type: "PROG/P"}}}, nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/search", map[string]string{
			"object_name": "ZHELL*",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeSuccess[types.ADTSearchResult](t, rec)
		if len(data.Objects) != 1 || data.Objects[0].Name != "ZHELLO_WORLD" {
			t.Errorf("unexpected results: %+v", data.Objects)
		}
	})

	t.Run("missing pattern", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/search", map[string]string{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
}

// --- objects/list (regression coverage for the package-filter fix) ---

func TestListObjectsHandler(t *testing.T) {
	t.Run("lists package contents", func(t *testing.T) {
		fake := &fakeADTClient{
			getPackageContentsFn: func(ctx context.Context, name string) (*types.ADTPackage, error) {
				if name != "$TMP" {
					t.Errorf("expected package $TMP, got %q", name)
				}
				return &types.ADTPackage{Name: name, Objects: []types.ADTObject{
					{Name: "ZHELLO_WORLD", Type: "PROG/P"},
					{Name: "ZCL_DEMO", Type: "CLAS/OC"},
				}}, nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/list", map[string]string{
			"package": "$tmp",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeSuccess[[]types.ADTObject](t, rec)
		if len(data) != 2 {
			t.Fatalf("expected 2 objects, got %d", len(data))
		}
	})

	t.Run("filters package contents by type", func(t *testing.T) {
		fake := &fakeADTClient{
			getPackageContentsFn: func(ctx context.Context, name string) (*types.ADTPackage, error) {
				return &types.ADTPackage{Name: name, Objects: []types.ADTObject{
					{Name: "ZHELLO_WORLD", Type: "PROG/P"},
					{Name: "ZHELLO_GO_TEST", Type: "PROG/P"},
					{Name: "ZCL_DEMO", Type: "CLAS/OC"},
				}}, nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/list", map[string]string{
			"package":     "$TMP",
			"object_type": "PROG",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeSuccess[[]types.ADTObject](t, rec)
		if len(data) != 2 {
			t.Fatalf("expected 2 PROG objects, got %d: %+v", len(data), data)
		}
		for _, obj := range data {
			if obj.Type != "PROG/P" {
				t.Errorf("unexpected object leaked through type filter: %+v", obj)
			}
		}
	})

	t.Run("lists packages by pattern", func(t *testing.T) {
		fake := &fakeADTClient{
			listPackagesFn: func(ctx context.Context, pattern string) ([]types.ADTPackage, error) {
				if pattern != "Z*" {
					t.Errorf("expected pattern Z*, got %q", pattern)
				}
				return []types.ADTPackage{{Name: "ZDEMO"}}, nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/list", map[string]string{
			"object_type": "packages",
			"object_name": "Z*",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeSuccess[[]types.ADTPackage](t, rec)
		if len(data) != 1 || data[0].Name != "ZDEMO" {
			t.Errorf("unexpected packages: %+v", data)
		}
	})

	t.Run("requires package when not listing package names", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/list", map[string]string{
			"object_type": "PROG",
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

// --- objects/activate ---

func TestActivateObjectHandler(t *testing.T) {
	t.Run("activates without source", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/activate", map[string]string{
			"object_type": "program",
			"object_name": "zfoo",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeSuccess[map[string]any](t, rec)
		activated, _ := data["activated"].(bool)
		if !activated {
			t.Errorf("expected activated=true in response")
		}
	})

	t.Run("saves source before activating", func(t *testing.T) {
		var saved bool
		fake := &fakeADTClient{
			updateProgramFn: func(ctx context.Context, name, source string) error {
				saved = true
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/activate", map[string]string{
			"object_type": "program",
			"object_name": "zfoo",
			"source":      "REPORT zfoo.",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if !saved {
			t.Errorf("expected UpdateProgram to be called before activation")
		}
	})
}

// --- service-bindings/publish ---

func TestPublishServiceBindingHandler(t *testing.T) {
	t.Run("publishes", func(t *testing.T) {
		var gotName string
		fake := &fakeADTClient{
			publishServiceBindingFn: func(ctx context.Context, name string) (*types.ServiceBindingPublishResult, error) {
				gotName = name
				return &types.ServiceBindingPublishResult{Severity: "OK", ShortText: name + " published locally"}, nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/service-bindings/publish", map[string]string{
			"object_name": "zabpb_sb1",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if gotName != "ZABPB_SB1" {
			t.Errorf("expected uppercased object name passed through, got %q", gotName)
		}
		data := decodeSuccess[types.ServiceBindingPublishResult](t, rec)
		if data.Severity != "OK" {
			t.Errorf("expected severity OK, got %q", data.Severity)
		}
	})

	t.Run("missing object_name", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/service-bindings/publish", map[string]string{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodGet, "/api/v1/service-bindings/publish", nil)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d", rec.Code)
		}
	})
}

// --- syntax-check, format, completion, navigation ---

func TestSyntaxCheckHandler(t *testing.T) {
	rs := newTestServer(t, &fakeADTClient{})
	rec := doJSON(t, rs, http.MethodPost, "/api/v1/syntax-check", map[string]string{
		"object_type": "program",
		"object_name": "zfoo",
		"source":      "REPORT zfoo.",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestFormatSourceHandler(t *testing.T) {
	rs := newTestServer(t, &fakeADTClient{})
	rec := doJSON(t, rs, http.MethodPost, "/api/v1/format", map[string]string{
		"source": "report zfoo.",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	data := decodeSuccess[map[string]string](t, rec)
	if data["source"] != "report zfoo." {
		t.Errorf("expected passthrough formatting from fake, got %q", data["source"])
	}
}

func TestCompletionHandler(t *testing.T) {
	rs := newTestServer(t, &fakeADTClient{})
	rec := doJSON(t, rs, http.MethodPost, "/api/v1/completion", map[string]any{
		"object_type": "program",
		"object_name": "zfoo",
		"source":      "REPORT zfoo.",
		"line":        1,
		"column":      1,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestNavigationHandler(t *testing.T) {
	rs := newTestServer(t, &fakeADTClient{})
	rec := doJSON(t, rs, http.MethodPost, "/api/v1/navigation", map[string]any{
		"object_type": "program",
		"object_name": "zfoo",
		"source":      "REPORT zfoo.",
		"line":        1,
		"column":      1,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- unit-tests, transports ---

func TestUnitTestsHandler(t *testing.T) {
	rs := newTestServer(t, &fakeADTClient{})
	rec := doJSON(t, rs, http.MethodPost, "/api/v1/unit-tests", map[string]string{
		"object_type": "class",
		"object_name": "zcl_demo",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	data := decodeSuccess[types.UnitTestResult](t, rec)
	if !data.AllPassed {
		t.Errorf("expected AllPassed from fake default")
	}
}

func TestTransportInfoHandler(t *testing.T) {
	rs := newTestServer(t, &fakeADTClient{})
	rec := doJSON(t, rs, http.MethodPost, "/api/v1/transports/info", map[string]string{
		"object_type": "program",
		"object_name": "zfoo",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateTransportHandler(t *testing.T) {
	t.Run("creates transport", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/transports/create", map[string]string{
			"object_type": "program",
			"object_name": "zfoo",
			"description": "test transport",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("missing description", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/transports/create", map[string]string{
			"object_type": "program",
			"object_name": "zfoo",
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
}

// --- system/connect, health, version ---

func TestConnectHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{authenticated: true})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/system/connect", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("connection failure", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{testConnErr: context.DeadlineExceeded})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/system/connect", nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestHealthHandler(t *testing.T) {
	rs := newTestServer(t, &fakeADTClient{authenticated: true})
	rec := doJSON(t, rs, http.MethodGet, "/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if body["adt_status"] != "connected" {
		t.Errorf("expected adt_status=connected, got %v", body["adt_status"])
	}
}

func TestVersionHandler(t *testing.T) {
	rs := newTestServer(t, &fakeADTClient{})
	rec := doJSON(t, rs, http.MethodGet, "/version", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

// --- /api/v1/activate alias (A) ---

func TestActivateAlias(t *testing.T) {
	rs := newTestServer(t, &fakeADTClient{})
	rec := doJSON(t, rs, http.MethodPost, "/api/v1/activate", map[string]string{
		"object_type": "program",
		"object_name": "zfoo",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	data := decodeSuccess[map[string]any](t, rec)
	if _, ok := data["activated"]; !ok {
		t.Errorf("response missing 'activated' field; got %v", data)
	}
}

// --- save-mode objects/create (B) ---

func TestCreateObjectHandler_SaveMode(t *testing.T) {
	t.Run("source without description triggers save", func(t *testing.T) {
		var savedName, savedSource string
		fake := &fakeADTClient{
			updateProgramFn: func(_ context.Context, name, source string) error {
				savedName = name
				savedSource = source
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/create", map[string]string{
			"object_type": "program",
			"object_name": "zfoo",
			"source":      "REPORT zfoo.",
			// description intentionally omitted
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeSuccess[map[string]any](t, rec)
		if created, _ := data["created"].(bool); created {
			t.Error("expected created=false for save mode")
		}
		if si, _ := data["source_inserted"].(bool); !si {
			t.Error("expected source_inserted=true")
		}
		if savedName != "ZFOO" || savedSource != "REPORT zfoo." {
			t.Errorf("UpdateProgram called with name=%q source=%q", savedName, savedSource)
		}
	})

	t.Run("source with description triggers create", func(t *testing.T) {
		var createdName string
		fake := &fakeADTClient{
			createProgramFn: func(_ context.Context, name, _, _, _ string) error {
				createdName = name
				return nil
			},
		}
		rs := newTestServer(t, fake)
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/objects/create", map[string]string{
			"object_type": "program",
			"object_name": "zbar",
			"description": "test program",
			"source":      "REPORT zbar.",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeSuccess[map[string]any](t, rec)
		if created, _ := data["created"].(bool); !created {
			t.Error("expected created=true for create mode")
		}
		if createdName != "ZBAR" {
			t.Errorf("CreateProgram called with name=%q", createdName)
		}
	})
}

// --- packages/contents (C) ---

func TestPackageContentsHandler(t *testing.T) {
	t.Run("returns nodes and objectTypes", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/packages/contents", map[string]string{
			"package_name": "ztest",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeSuccess[types.PackageContentsResult](t, rec)
		if len(data.Nodes) == 0 {
			t.Error("expected at least one node from fake")
		}
		if len(data.ObjectTypes) == 0 {
			t.Error("expected at least one objectType from fake")
		}
	})

	t.Run("missing package_name returns 400", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/packages/contents", map[string]string{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
}

// --- removed AI endpoints stay gone ---

func TestRemovedAIHandlersReturn410(t *testing.T) {
	rs := newTestServer(t, &fakeADTClient{})
	for _, path := range []string{
		"/api/v1/ai/analyze",
		"/api/v1/ai/review",
		"/api/v1/ai/optimize",
		"/api/v1/ai/create",
		"/generate-code",
		"/generate-code-stream",
	} {
		rec := doJSON(t, rs, http.MethodPost, path, nil)
		if rec.Code != http.StatusGone {
			t.Errorf("%s: expected 410 Gone, got %d", path, rec.Code)
		}
	}
}
