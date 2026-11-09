// Package types defines the ADTClient interface and shared data structures
// for interacting with SAP ABAP Development Tools (ADT) REST APIs.
// These types are shared across the CLI, REST server, and LSP server.
package types

import (
	"context"
	"encoding/xml"
	"time"
)

// ADT Response structures - shared between CLI and REST
type ADTObject struct {
	Name        string `json:"name" xml:"name,attr"`
	Type        string `json:"type" xml:"type,attr"`
	Description string `json:"description" xml:"description,attr"`
	Package     string `json:"package" xml:"packageName,attr"`
	Responsible string `json:"responsible" xml:"responsible,attr"`
	CreatedBy   string `json:"created_by" xml:"createdBy,attr"`
	CreatedOn   string `json:"created_on" xml:"createdOn,attr"`
	ChangedBy   string `json:"changed_by" xml:"changedBy,attr"`
	ChangedOn   string `json:"changed_on" xml:"changedOn,attr"`
}

type ADTPackage struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Objects     []ADTObject `json:"objects"`
}

type ADTSourceCode struct {
	ObjectName string `json:"object_name"`
	ObjectType string `json:"object_type"`
	Source     string `json:"source"`
	Version    string `json:"version"`
	ETag       string `json:"etag"`
}

type ADTSearchResult struct {
	XMLName xml.Name    `xml:"http://www.sap.com/adt/core objectReferences"` // root element with namespace
	Objects []ADTObject `xml:"objectReference"`
	Total   int         `xml:"total,attr"`
}

type ADTNode struct {
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Children    []ADTNode `json:"children,omitempty"`
}

type ADTTransport struct {
	RequestID   string      `json:"request_id"`
	Description string      `json:"description"`
	Status      string      `json:"status"`
	Owner       string      `json:"owner"`
	Objects     []ADTObject `json:"objects"`
}

type ADTTransportEntry struct {
	Number      string `json:"number"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
	Status      string `json:"status"` // "D"=modifiable, "R"=released
	Type        string `json:"type"`   // "K"=workbench, "W"=customizing
	Target      string `json:"target"`
	Date        string `json:"date"`
}

type ADTTransportInfo struct {
	ObjectName string              `json:"object_name"`
	Package    string              `json:"package"`
	Transports []ADTTransportEntry `json:"transports"`
}

// ADT Configuration
type ADTConfig struct {
	Host            string        `json:"host"`
	Client          string        `json:"client"`
	Username        string        `json:"username"`
	Password        string        `json:"password"`
	Language        string        `json:"language"`
	AllowSelfSigned bool          `json:"allow_self_signed"`
	ConnectTimeout  time.Duration `json:"connect_timeout"`
	RequestTimeout  time.Duration `json:"request_timeout"`
	Debug           bool          `json:"debug"`
}

// Additional data structures for extended services
type ADTTransactionInfo struct {
	TransactionCode string            `json:"transaction_code"`
	Description     string            `json:"description"`
	Package         string            `json:"package"`
	Application     string            `json:"application"`
	Program         string            `json:"program"`
	Properties      map[string]string `json:"properties"`
}

type ADTTableData struct {
	TableName string           `json:"table_name"`
	RowCount  int              `json:"row_count"`
	Columns   []ADTTableColumn `json:"columns"`
	Rows      []map[string]any `json:"rows"`
}

type ADTTableColumn struct {
	Name     string `json:"name"`
	DataType string `json:"data_type"`
	Length   int    `json:"length"`
	Decimals int    `json:"decimals"`
}

type ADTTypeInfo struct {
	TypeName    string         `json:"type_name"`
	TypeKind    string         `json:"type_kind"` // "DOMAIN", "DATA_ELEMENT", etc.
	Description string         `json:"description"`
	Source      string         `json:"source"`
	Properties  map[string]any `json:"properties"`
}

// ActivationResult holds the result of an object activation
type ActivationResult struct {
	ObjectName string              `json:"object_name"`
	ObjectType string              `json:"object_type"`
	Success    bool                `json:"success"`
	Messages   []ActivationMessage `json:"messages,omitempty"`
}

// ActivationMessage represents a single message from the activation response
type ActivationMessage struct {
	Severity string `json:"severity"` // "error", "warning", "info"
	Text     string `json:"text"`
	Line     int    `json:"line,omitempty"`
}

// UnitTestResult holds the result of running ABAP unit tests
type UnitTestResult struct {
	ObjectName  string            `json:"object_name"`
	TotalTests  int               `json:"total_tests"`
	Passed      int               `json:"passed"`
	Failed      int               `json:"failed"`
	AllPassed   bool              `json:"all_passed"`
	TestClasses []TestClassResult `json:"test_classes"`
}

// TestClassResult holds results for a single test class
type TestClassResult struct {
	Name    string             `json:"name"`
	Methods []TestMethodResult `json:"methods"`
}

// TestMethodResult holds result for a single test method
type TestMethodResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "passed", "failed", "error"
	Message string `json:"message,omitempty"`
}

// Session management
type SessionType string

const (
	SessionStateful  SessionType = "stateful"
	SessionStateless SessionType = "stateless"
)

// SourceReader retrieves ABAP object source.
type SourceReader interface {
	GetProgram(ctx context.Context, name string) (*ADTSourceCode, error)
	GetClass(ctx context.Context, name string) (*ADTSourceCode, error)
	GetInclude(ctx context.Context, name string) (*ADTSourceCode, error)
	GetInterface(ctx context.Context, name string) (*ADTSourceCode, error)
	GetStructure(ctx context.Context, name string) (*ADTSourceCode, error)
	GetTable(ctx context.Context, name string) (*ADTSourceCode, error)
	GetFunction(ctx context.Context, functionName, functionGroup string) (*ADTSourceCode, error)
	GetFunctionGroup(ctx context.Context, name string) (*ADTSourceCode, error)
	GetDDLSource(ctx context.Context, name string) (*ADTSourceCode, error)
	GetSRVDSource(ctx context.Context, name string) (*ADTSourceCode, error)
	GetObjectSource(ctx context.Context, objectType, objectName string) (string, error)
	CheckObjectExists(ctx context.Context, objectType, objectName string) (bool, error)
}

// SourceWriter creates and updates ABAP objects.
type SourceWriter interface {
	CreateProgram(ctx context.Context, name, description, packageName, source string) error
	CreateClass(ctx context.Context, name, description, packageName, source string) error
	CreateInclude(ctx context.Context, name, description, source string) error
	CreateInterface(ctx context.Context, name, description, source string) error
	CreateStructure(ctx context.Context, name, description, source string) error
	CreateTable(ctx context.Context, name, description, source string) error
	CreateFunctionGroup(ctx context.Context, name, description, source string) error
	CreateFunction(ctx context.Context, name, functionGroup, description, source string) error
	CreateDDLS(ctx context.Context, name, description, source string) error
	CreateSRVD(ctx context.Context, name, description, source string) error
	UpdateProgram(ctx context.Context, name, source string) error
	UpdateClass(ctx context.Context, name, source string) error
	UpdateInclude(ctx context.Context, name, source string) error
	UpdateInterface(ctx context.Context, name, source string) error
	UpdateFunction(ctx context.Context, functionName, functionGroup, source string) error
	UpdateFunctionGroup(ctx context.Context, name, source string) error
	UpdateTable(ctx context.Context, name, source string) error
	UpdateStructure(ctx context.Context, name, source string) error
	UpdateDDLS(ctx context.Context, name, source string) error
	UpdateSRVD(ctx context.Context, name, source string) error
}

// PackageNode is a single entry returned by the nodestructure endpoint.
// It carries a URI and expandable flag, enabling tree navigation in the editor.
type PackageNode struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Expandable  bool   `json:"expandable"`
	URI         string `json:"uri"`
}

// PackageObjectType describes a category of objects present in a package.
type PackageObjectType struct {
	Type  string `json:"type"`
	Label string `json:"label"`
}

// PackageContentsResult is the response shape expected by abaper-editor's Explorer tree.
type PackageContentsResult struct {
	Nodes       []PackageNode       `json:"nodes"`
	ObjectTypes []PackageObjectType `json:"objectTypes"`
}

// DomainProperties describes a DDIC domain (DOMA/DD). Unlike source-text
// objects, domains are defined by structured properties, not ABAP source.
type DomainProperties struct {
	Description    string `json:"description,omitempty"`
	DataType       string `json:"data_type"`               // CHAR, NUMC, DEC, INT4, ...
	Length         int    `json:"length"`                  // internal length
	Decimals       int    `json:"decimals,omitempty"`      // for DEC/QUAN/CURR
	OutputLength   int    `json:"output_length,omitempty"` // defaults to Length when 0
	ConversionExit string `json:"conversion_exit,omitempty"`
	LowerCase      bool   `json:"lowercase,omitempty"`
}

// DataElementProperties describes a DDIC data element (DTEL/DE). A data element
// is either domain-based (set DomainName) or built on a predefined ABAP type
// (set DataType/Length/Decimals).
type DataElementProperties struct {
	Description  string `json:"description,omitempty"`
	DomainName   string `json:"domain_name,omitempty"` // typeKind=domain when set
	DataType     string `json:"data_type,omitempty"`   // typeKind=predefinedAbapType otherwise
	Length       int    `json:"length,omitempty"`
	Decimals     int    `json:"decimals,omitempty"`
	ShortLabel   string `json:"short_label,omitempty"`
	MediumLabel  string `json:"medium_label,omitempty"`
	LongLabel    string `json:"long_label,omitempty"`
	HeadingLabel string `json:"heading_label,omitempty"`
}

// DDICPropertyWriter creates and updates DDIC objects that are defined by
// structured properties rather than ABAP source text (domains, data elements).
// These follow a distinct create-shell -> lock -> set-properties -> unlock ->
// activate flow (activation must follow unlock, unlike source-text objects).
type DDICPropertyWriter interface {
	CreateDomain(ctx context.Context, name string, props DomainProperties) error
	UpdateDomain(ctx context.Context, name string, props DomainProperties) error
	CreateDataElement(ctx context.Context, name string, props DataElementProperties) error
	UpdateDataElement(ctx context.Context, name string, props DataElementProperties) error
}

// ServiceBindingProperties describes a Service Binding (SRVB/SVB): the
// runtime OData exposure of a Service Definition. Like domains/data
// elements, this is structured metadata, not ABAP source text.
type ServiceBindingProperties struct {
	Description           string `json:"description,omitempty"`
	ServiceDefinitionName string `json:"service_definition_name"`    // the SRVD/SRV this binding exposes
	BindingVersion        string `json:"binding_version,omitempty"`  // "V2" or "V4" (OData version); defaults to "V4"
	BindingCategory       string `json:"binding_category,omitempty"` // "0"=UI, "1"=Web API; defaults to "0"
}

// ADTServiceBinding is the parsed result of reading a Service Binding.
type ADTServiceBinding struct {
	Name                  string `json:"name"`
	Description           string `json:"description"`
	ServiceDefinitionName string `json:"service_definition_name"`
	BindingVersion        string `json:"binding_version"`
	BindingCategory       string `json:"binding_category"`
	Version               string `json:"version"` // active/inactive
	Published             bool   `json:"published"`
}

// ServiceBindingPublishResult reports the outcome of publishing a Service
// Binding, registering its OData service for runtime consumption.
type ServiceBindingPublishResult struct {
	Severity  string `json:"severity"`
	ShortText string `json:"short_text"`
	LongText  string `json:"long_text,omitempty"`
}

// RAPServiceWriter creates, updates and publishes RAP-exposed OData
// services (Service Bindings) that expose CDS views via a Service
// Definition. Follows the same create -> lock -> set-properties -> unlock
// -> activate flow as DDICPropertyWriter, plus a separate publish step to
// register the runtime OData endpoint.
type RAPServiceWriter interface {
	GetServiceBinding(ctx context.Context, name string) (*ADTServiceBinding, error)
	CreateServiceBinding(ctx context.Context, name string, props ServiceBindingProperties) error
	UpdateServiceBinding(ctx context.Context, name string, props ServiceBindingProperties) error
	PublishServiceBinding(ctx context.Context, name string) (*ServiceBindingPublishResult, error)
}

// PackageBrowser searches and navigates package contents.
type PackageBrowser interface {
	SearchObjects(ctx context.Context, pattern string, objectTypes []string) (*ADTSearchResult, error)
	ListPackages(ctx context.Context, pattern string) ([]ADTPackage, error)
	GetPackageContents(ctx context.Context, name string) (*ADTPackage, error)
	GetNodeContents(ctx context.Context, packageName string) (*PackageContentsResult, error)
}

// ObjectActivator activates objects and runs tests.
type ObjectActivator interface {
	ActivateObject(ctx context.Context, objectType, objectName string) (*ActivationResult, error)
	RunUnitTests(ctx context.Context, objectType, objectName string) (*UnitTestResult, error)
}

// LangFeatures provides LSP-style language intelligence.
type LangFeatures interface {
	SyntaxCheck(ctx context.Context, objectType, objectName, source string) (*SyntaxCheckResult, error)
	GetCompletionProposals(ctx context.Context, objectType, objectName, source string, line, col int) ([]CompletionProposal, error)
	GetNavigationTarget(ctx context.Context, objectType, objectName, source string, line, col int) (*NavigationTarget, error)
	FormatSource(ctx context.Context, source string) (string, error)
}

// SessionManager controls client lifecycle.
type SessionManager interface {
	Authenticate() error
	IsAuthenticated() bool
	TestConnection() error
	SetSessionType(sessionType SessionType)
}

// ADTClient is the full capability interface satisfied by ADTClientImpl.
// Prefer the smaller focused interfaces (SourceReader, LangFeatures, etc.)
// when a consumer only needs a subset of operations.
type ADTClient interface {
	SessionManager
	SourceReader
	SourceWriter
	DDICPropertyWriter
	RAPServiceWriter
	PackageBrowser
	ObjectActivator
	LangFeatures
	// Extended operations
	GetTypeInfo(ctx context.Context, typeName string) (*ADTTypeInfo, error)
	GetTransaction(ctx context.Context, transactionName string) (*ADTTransactionInfo, error)
	GetTableContents(ctx context.Context, tableName string, maxRows int) (*ADTTableData, error)
	GetTransports(ctx context.Context) ([]ADTTransport, error)
	GetTransportInfo(ctx context.Context, objectType, objectName string) (*ADTTransportInfo, error)
	CreateTransport(ctx context.Context, objectType, objectName, description, packageName string) (string, error)
}
