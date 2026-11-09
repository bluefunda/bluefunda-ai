package server

import (
	"context"

	"github.com/bluefunda/abaper/types"
)

// fakeADTClient is a scriptable stand-in for types.ADTClient used to unit
// test REST handlers without a live SAP system. Each field defaults to a
// zero-value success response; tests override only what they need.
type fakeADTClient struct {
	authenticated bool
	testConnErr   error

	getProgramFn            func(ctx context.Context, name string) (*types.ADTSourceCode, error)
	getClassFn              func(ctx context.Context, name string) (*types.ADTSourceCode, error)
	getIncludeFn            func(ctx context.Context, name string) (*types.ADTSourceCode, error)
	getInterfaceFn          func(ctx context.Context, name string) (*types.ADTSourceCode, error)
	getStructureFn          func(ctx context.Context, name string) (*types.ADTSourceCode, error)
	getTableFn              func(ctx context.Context, name string) (*types.ADTSourceCode, error)
	getFunctionFn           func(ctx context.Context, functionName, functionGroup string) (*types.ADTSourceCode, error)
	getFunctionGroupFn      func(ctx context.Context, name string) (*types.ADTSourceCode, error)
	getDDLSourceFn          func(ctx context.Context, name string) (*types.ADTSourceCode, error)
	createIncludeFn         func(ctx context.Context, name, description, source string) error
	createInterfaceFn       func(ctx context.Context, name, description, source string) error
	createStructureFn       func(ctx context.Context, name, description, source string) error
	createTableFn           func(ctx context.Context, name, description, source string) error
	createFunctionGroupFn   func(ctx context.Context, name, description, source string) error
	updateIncludeFn         func(ctx context.Context, name, source string) error
	updateInterfaceFn       func(ctx context.Context, name, source string) error
	updateFunctionGroupFn   func(ctx context.Context, name, source string) error
	getSRVDSourceFn         func(ctx context.Context, name string) (*types.ADTSourceCode, error)
	getPackageContentsFn    func(ctx context.Context, name string) (*types.ADTPackage, error)
	getNodeContentsFn       func(ctx context.Context, name string) (*types.PackageContentsResult, error)
	listPackagesFn          func(ctx context.Context, pattern string) ([]types.ADTPackage, error)
	searchObjectsFn         func(ctx context.Context, pattern string, objectTypes []string) (*types.ADTSearchResult, error)
	updateProgramFn         func(ctx context.Context, name, source string) error
	updateClassFn           func(ctx context.Context, name, source string) error
	updateFunctionFn        func(ctx context.Context, functionName, functionGroup, source string) error
	updateTableFn           func(ctx context.Context, name, source string) error
	updateStructureFn       func(ctx context.Context, name, source string) error
	updateDDLSFn            func(ctx context.Context, name, source string) error
	createProgramFn         func(ctx context.Context, name, description, packageName, source string) error
	createClassFn           func(ctx context.Context, name, description, packageName, source string) error
	createFunctionFn        func(ctx context.Context, name, functionGroup, description, source string) error
	createDDLSFn            func(ctx context.Context, name, description, source string) error
	createSRVDFn            func(ctx context.Context, name, description, source string) error
	updateSRVDFn            func(ctx context.Context, name, source string) error
	createDomainFn          func(ctx context.Context, name string, props types.DomainProperties) error
	updateDomainFn          func(ctx context.Context, name string, props types.DomainProperties) error
	createDataElementFn     func(ctx context.Context, name string, props types.DataElementProperties) error
	updateDataElementFn     func(ctx context.Context, name string, props types.DataElementProperties) error
	getServiceBindingFn     func(ctx context.Context, name string) (*types.ADTServiceBinding, error)
	createServiceBindingFn  func(ctx context.Context, name string, props types.ServiceBindingProperties) error
	updateServiceBindingFn  func(ctx context.Context, name string, props types.ServiceBindingProperties) error
	publishServiceBindingFn func(ctx context.Context, name string) (*types.ServiceBindingPublishResult, error)
	getTypeInfoFn           func(ctx context.Context, typeName string) (*types.ADTTypeInfo, error)
	activateObjectFn        func(ctx context.Context, objectType, objectName string) (*types.ActivationResult, error)
	runUnitTestsFn          func(ctx context.Context, objectType, objectName string) (*types.UnitTestResult, error)
	syntaxCheckFn           func(ctx context.Context, objectType, objectName, source string) (*types.SyntaxCheckResult, error)
	formatSourceFn          func(ctx context.Context, source string) (string, error)
	completionFn            func(ctx context.Context, objectType, objectName, source string, line, col int) ([]types.CompletionProposal, error)
	navigationFn            func(ctx context.Context, objectType, objectName, source string, line, col int) (*types.NavigationTarget, error)
	transportInfoFn         func(ctx context.Context, objectType, objectName string) (*types.ADTTransportInfo, error)
	createTransportFn       func(ctx context.Context, objectType, objectName, description, packageName string) (string, error)
}

func (f *fakeADTClient) Authenticate() error              { return nil }
func (f *fakeADTClient) IsAuthenticated() bool            { return f.authenticated }
func (f *fakeADTClient) TestConnection() error            { return f.testConnErr }
func (f *fakeADTClient) SetSessionType(types.SessionType) {}

func (f *fakeADTClient) GetProgram(ctx context.Context, name string) (*types.ADTSourceCode, error) {
	if f.getProgramFn != nil {
		return f.getProgramFn(ctx, name)
	}
	return &types.ADTSourceCode{ObjectName: name, ObjectType: "PROGRAM"}, nil
}
func (f *fakeADTClient) GetClass(ctx context.Context, name string) (*types.ADTSourceCode, error) {
	if f.getClassFn != nil {
		return f.getClassFn(ctx, name)
	}
	return &types.ADTSourceCode{ObjectName: name, ObjectType: "CLASS"}, nil
}
func (f *fakeADTClient) GetInclude(ctx context.Context, name string) (*types.ADTSourceCode, error) {
	if f.getIncludeFn != nil {
		return f.getIncludeFn(ctx, name)
	}
	return &types.ADTSourceCode{ObjectName: name, ObjectType: "INCLUDE"}, nil
}
func (f *fakeADTClient) GetInterface(ctx context.Context, name string) (*types.ADTSourceCode, error) {
	if f.getInterfaceFn != nil {
		return f.getInterfaceFn(ctx, name)
	}
	return &types.ADTSourceCode{ObjectName: name, ObjectType: "INTERFACE"}, nil
}
func (f *fakeADTClient) GetStructure(ctx context.Context, name string) (*types.ADTSourceCode, error) {
	if f.getStructureFn != nil {
		return f.getStructureFn(ctx, name)
	}
	return &types.ADTSourceCode{ObjectName: name, ObjectType: "STRUCTURE"}, nil
}
func (f *fakeADTClient) GetTable(ctx context.Context, name string) (*types.ADTSourceCode, error) {
	if f.getTableFn != nil {
		return f.getTableFn(ctx, name)
	}
	return &types.ADTSourceCode{ObjectName: name, ObjectType: "TABLE"}, nil
}
func (f *fakeADTClient) GetFunction(ctx context.Context, functionName, functionGroup string) (*types.ADTSourceCode, error) {
	if f.getFunctionFn != nil {
		return f.getFunctionFn(ctx, functionName, functionGroup)
	}
	return &types.ADTSourceCode{ObjectName: functionName, ObjectType: "FUNCTION"}, nil
}
func (f *fakeADTClient) GetFunctionGroup(ctx context.Context, name string) (*types.ADTSourceCode, error) {
	if f.getFunctionGroupFn != nil {
		return f.getFunctionGroupFn(ctx, name)
	}
	return &types.ADTSourceCode{ObjectName: name, ObjectType: "FUNCTIONGROUP"}, nil
}
func (f *fakeADTClient) GetDDLSource(ctx context.Context, name string) (*types.ADTSourceCode, error) {
	if f.getDDLSourceFn != nil {
		return f.getDDLSourceFn(ctx, name)
	}
	return &types.ADTSourceCode{ObjectName: name, ObjectType: "DDLS"}, nil
}
func (f *fakeADTClient) GetSRVDSource(ctx context.Context, name string) (*types.ADTSourceCode, error) {
	if f.getSRVDSourceFn != nil {
		return f.getSRVDSourceFn(ctx, name)
	}
	return &types.ADTSourceCode{ObjectName: name, ObjectType: "SRVD"}, nil
}
func (f *fakeADTClient) GetObjectSource(ctx context.Context, objectType, objectName string) (string, error) {
	return "", nil
}
func (f *fakeADTClient) CheckObjectExists(ctx context.Context, objectType, objectName string) (bool, error) {
	return true, nil
}

func (f *fakeADTClient) CreateProgram(ctx context.Context, name, description, packageName, source string) error {
	if f.createProgramFn != nil {
		return f.createProgramFn(ctx, name, description, packageName, source)
	}
	return nil
}
func (f *fakeADTClient) CreateClass(ctx context.Context, name, description, packageName, source string) error {
	if f.createClassFn != nil {
		return f.createClassFn(ctx, name, description, packageName, source)
	}
	return nil
}
func (f *fakeADTClient) CreateInclude(ctx context.Context, name, description, source string) error {
	if f.createIncludeFn != nil {
		return f.createIncludeFn(ctx, name, description, source)
	}
	return nil
}
func (f *fakeADTClient) CreateInterface(ctx context.Context, name, description, source string) error {
	if f.createInterfaceFn != nil {
		return f.createInterfaceFn(ctx, name, description, source)
	}
	return nil
}
func (f *fakeADTClient) CreateStructure(ctx context.Context, name, description, source string) error {
	if f.createStructureFn != nil {
		return f.createStructureFn(ctx, name, description, source)
	}
	return nil
}
func (f *fakeADTClient) CreateTable(ctx context.Context, name, description, source string) error {
	if f.createTableFn != nil {
		return f.createTableFn(ctx, name, description, source)
	}
	return nil
}
func (f *fakeADTClient) CreateFunctionGroup(ctx context.Context, name, description, source string) error {
	if f.createFunctionGroupFn != nil {
		return f.createFunctionGroupFn(ctx, name, description, source)
	}
	return nil
}
func (f *fakeADTClient) CreateFunction(ctx context.Context, name, functionGroup, description, source string) error {
	if f.createFunctionFn != nil {
		return f.createFunctionFn(ctx, name, functionGroup, description, source)
	}
	return nil
}
func (f *fakeADTClient) CreateDDLS(ctx context.Context, name, description, source string) error {
	if f.createDDLSFn != nil {
		return f.createDDLSFn(ctx, name, description, source)
	}
	return nil
}
func (f *fakeADTClient) CreateSRVD(ctx context.Context, name, description, source string) error {
	if f.createSRVDFn != nil {
		return f.createSRVDFn(ctx, name, description, source)
	}
	return nil
}
func (f *fakeADTClient) UpdateProgram(ctx context.Context, name, source string) error {
	if f.updateProgramFn != nil {
		return f.updateProgramFn(ctx, name, source)
	}
	return nil
}
func (f *fakeADTClient) UpdateClass(ctx context.Context, name, source string) error {
	if f.updateClassFn != nil {
		return f.updateClassFn(ctx, name, source)
	}
	return nil
}
func (f *fakeADTClient) UpdateInclude(ctx context.Context, name, source string) error {
	if f.updateIncludeFn != nil {
		return f.updateIncludeFn(ctx, name, source)
	}
	return nil
}
func (f *fakeADTClient) UpdateInterface(ctx context.Context, name, source string) error {
	if f.updateInterfaceFn != nil {
		return f.updateInterfaceFn(ctx, name, source)
	}
	return nil
}
func (f *fakeADTClient) UpdateFunction(ctx context.Context, functionName, functionGroup, source string) error {
	if f.updateFunctionFn != nil {
		return f.updateFunctionFn(ctx, functionName, functionGroup, source)
	}
	return nil
}
func (f *fakeADTClient) UpdateFunctionGroup(ctx context.Context, name, source string) error {
	if f.updateFunctionGroupFn != nil {
		return f.updateFunctionGroupFn(ctx, name, source)
	}
	return nil
}
func (f *fakeADTClient) UpdateTable(ctx context.Context, name, source string) error {
	if f.updateTableFn != nil {
		return f.updateTableFn(ctx, name, source)
	}
	return nil
}
func (f *fakeADTClient) UpdateStructure(ctx context.Context, name, source string) error {
	if f.updateStructureFn != nil {
		return f.updateStructureFn(ctx, name, source)
	}
	return nil
}
func (f *fakeADTClient) UpdateDDLS(ctx context.Context, name, source string) error {
	if f.updateDDLSFn != nil {
		return f.updateDDLSFn(ctx, name, source)
	}
	return nil
}
func (f *fakeADTClient) UpdateSRVD(ctx context.Context, name, source string) error {
	if f.updateSRVDFn != nil {
		return f.updateSRVDFn(ctx, name, source)
	}
	return nil
}
func (f *fakeADTClient) CreateDomain(ctx context.Context, name string, props types.DomainProperties) error {
	if f.createDomainFn != nil {
		return f.createDomainFn(ctx, name, props)
	}
	return nil
}
func (f *fakeADTClient) UpdateDomain(ctx context.Context, name string, props types.DomainProperties) error {
	if f.updateDomainFn != nil {
		return f.updateDomainFn(ctx, name, props)
	}
	return nil
}
func (f *fakeADTClient) CreateDataElement(ctx context.Context, name string, props types.DataElementProperties) error {
	if f.createDataElementFn != nil {
		return f.createDataElementFn(ctx, name, props)
	}
	return nil
}
func (f *fakeADTClient) UpdateDataElement(ctx context.Context, name string, props types.DataElementProperties) error {
	if f.updateDataElementFn != nil {
		return f.updateDataElementFn(ctx, name, props)
	}
	return nil
}

func (f *fakeADTClient) GetServiceBinding(ctx context.Context, name string) (*types.ADTServiceBinding, error) {
	if f.getServiceBindingFn != nil {
		return f.getServiceBindingFn(ctx, name)
	}
	return &types.ADTServiceBinding{Name: name}, nil
}
func (f *fakeADTClient) CreateServiceBinding(ctx context.Context, name string, props types.ServiceBindingProperties) error {
	if f.createServiceBindingFn != nil {
		return f.createServiceBindingFn(ctx, name, props)
	}
	return nil
}
func (f *fakeADTClient) UpdateServiceBinding(ctx context.Context, name string, props types.ServiceBindingProperties) error {
	if f.updateServiceBindingFn != nil {
		return f.updateServiceBindingFn(ctx, name, props)
	}
	return nil
}
func (f *fakeADTClient) PublishServiceBinding(ctx context.Context, name string) (*types.ServiceBindingPublishResult, error) {
	if f.publishServiceBindingFn != nil {
		return f.publishServiceBindingFn(ctx, name)
	}
	return &types.ServiceBindingPublishResult{Severity: "OK"}, nil
}

func (f *fakeADTClient) SearchObjects(ctx context.Context, pattern string, objectTypes []string) (*types.ADTSearchResult, error) {
	if f.searchObjectsFn != nil {
		return f.searchObjectsFn(ctx, pattern, objectTypes)
	}
	return &types.ADTSearchResult{}, nil
}
func (f *fakeADTClient) ListPackages(ctx context.Context, pattern string) ([]types.ADTPackage, error) {
	if f.listPackagesFn != nil {
		return f.listPackagesFn(ctx, pattern)
	}
	return nil, nil
}
func (f *fakeADTClient) GetPackageContents(ctx context.Context, name string) (*types.ADTPackage, error) {
	if f.getPackageContentsFn != nil {
		return f.getPackageContentsFn(ctx, name)
	}
	return &types.ADTPackage{Name: name}, nil
}
func (f *fakeADTClient) GetNodeContents(ctx context.Context, name string) (*types.PackageContentsResult, error) {
	if f.getNodeContentsFn != nil {
		return f.getNodeContentsFn(ctx, name)
	}
	return &types.PackageContentsResult{
		Nodes:       []types.PackageNode{{Name: name, Type: "DEVC/K", Expandable: true, URI: "/sap/bc/adt/packages/" + name}},
		ObjectTypes: []types.PackageObjectType{{Type: "DEVC/K", Label: "Packages"}},
	}, nil
}

func (f *fakeADTClient) ActivateObject(ctx context.Context, objectType, objectName string) (*types.ActivationResult, error) {
	if f.activateObjectFn != nil {
		return f.activateObjectFn(ctx, objectType, objectName)
	}
	return &types.ActivationResult{ObjectName: objectName, ObjectType: objectType, Success: true}, nil
}
func (f *fakeADTClient) RunUnitTests(ctx context.Context, objectType, objectName string) (*types.UnitTestResult, error) {
	if f.runUnitTestsFn != nil {
		return f.runUnitTestsFn(ctx, objectType, objectName)
	}
	return &types.UnitTestResult{ObjectName: objectName, AllPassed: true}, nil
}

func (f *fakeADTClient) SyntaxCheck(ctx context.Context, objectType, objectName, source string) (*types.SyntaxCheckResult, error) {
	if f.syntaxCheckFn != nil {
		return f.syntaxCheckFn(ctx, objectType, objectName, source)
	}
	return &types.SyntaxCheckResult{ObjectName: objectName, ObjectType: objectType}, nil
}
func (f *fakeADTClient) GetCompletionProposals(ctx context.Context, objectType, objectName, source string, line, col int) ([]types.CompletionProposal, error) {
	if f.completionFn != nil {
		return f.completionFn(ctx, objectType, objectName, source, line, col)
	}
	return nil, nil
}
func (f *fakeADTClient) GetNavigationTarget(ctx context.Context, objectType, objectName, source string, line, col int) (*types.NavigationTarget, error) {
	if f.navigationFn != nil {
		return f.navigationFn(ctx, objectType, objectName, source, line, col)
	}
	return &types.NavigationTarget{}, nil
}
func (f *fakeADTClient) FormatSource(ctx context.Context, source string) (string, error) {
	if f.formatSourceFn != nil {
		return f.formatSourceFn(ctx, source)
	}
	return source, nil
}

func (f *fakeADTClient) GetTypeInfo(ctx context.Context, typeName string) (*types.ADTTypeInfo, error) {
	if f.getTypeInfoFn != nil {
		return f.getTypeInfoFn(ctx, typeName)
	}
	return &types.ADTTypeInfo{TypeName: typeName}, nil
}
func (f *fakeADTClient) GetTransaction(ctx context.Context, transactionName string) (*types.ADTTransactionInfo, error) {
	return &types.ADTTransactionInfo{TransactionCode: transactionName}, nil
}
func (f *fakeADTClient) GetTableContents(ctx context.Context, tableName string, maxRows int) (*types.ADTTableData, error) {
	return &types.ADTTableData{TableName: tableName}, nil
}
func (f *fakeADTClient) GetTransports(ctx context.Context) ([]types.ADTTransport, error) {
	return nil, nil
}
func (f *fakeADTClient) GetTransportInfo(ctx context.Context, objectType, objectName string) (*types.ADTTransportInfo, error) {
	if f.transportInfoFn != nil {
		return f.transportInfoFn(ctx, objectType, objectName)
	}
	return &types.ADTTransportInfo{ObjectName: objectName}, nil
}
func (f *fakeADTClient) CreateTransport(ctx context.Context, objectType, objectName, description, packageName string) (string, error) {
	if f.createTransportFn != nil {
		return f.createTransportFn(ctx, objectType, objectName, description, packageName)
	}
	return "TR0001", nil
}

var _ types.ADTClient = (*fakeADTClient)(nil)
