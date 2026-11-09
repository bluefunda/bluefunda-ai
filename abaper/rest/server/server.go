package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bluefunda/abaper/rest/models"
	"github.com/bluefunda/abaper/types"
	"go.uber.org/zap"
)

// Config represents the application configuration
type Config struct {
	APIKey          string
	ADTHost         string
	ADTClient       string
	ADTUsername     string
	ADTPassword     string
	AllowSelfSigned bool
	Verbose         bool
	Quiet           bool
}

// RestServer handles REST API requests with CLI feature parity (no AI)
type RestServer struct {
	logger         *zap.Logger
	config         *Config
	pool           *Pool           // nil when running in single-system mode
	adtClient      types.ADTClient // static fallback client (single-system mode)
	sourceReader   types.SourceReader
	sourceWriter   types.SourceWriter
	packageBrowser types.PackageBrowser
}

// NewRestServer creates a new REST server instance with a static ADT client.
// Use NewRestServerWithPool for multi-system support.
func NewRestServer(config *Config, logger *zap.Logger, adtClient types.ADTClient) *RestServer {
	return &RestServer{
		logger:         logger.With(zap.String("component", "rest_server")),
		config:         config,
		adtClient:      adtClient,
		sourceReader:   adtClient,
		sourceWriter:   adtClient,
		packageBrowser: adtClient,
	}
}

// NewRestServerWithPool creates a REST server that selects an ADT client from
// the pool on each request using X-SAP-* headers, with a static fallback.
func NewRestServerWithPool(config *Config, logger *zap.Logger, pool *Pool, fallback types.ADTClient) *RestServer {
	rs := &RestServer{
		logger:    logger.With(zap.String("component", "rest_server")),
		config:    config,
		pool:      pool,
		adtClient: fallback,
	}
	if fallback != nil {
		rs.sourceReader = fallback
		rs.sourceWriter = fallback
		rs.packageBrowser = fallback
	}
	return rs
}

// clientForRequest returns the ADT client to use for this request.
// If X-SAP-* headers are present and a pool is configured, a pooled client is
// returned. Otherwise falls back to the static client.
func (rs *RestServer) clientForRequest(r *http.Request) (types.ADTClient, error) {
	host := r.Header.Get("X-SAP-Host")
	sapClient := r.Header.Get("X-SAP-Client")
	user := r.Header.Get("X-SAP-User")
	password := r.Header.Get("X-SAP-Password")

	if rs.pool != nil && host != "" && user != "" && password != "" {
		if sapClient == "" {
			sapClient = "100"
		}
		cfg := types.ADTConfig{
			Host:            host,
			Client:          sapClient,
			Username:        user,
			Password:        password,
			Language:        "EN",
			AllowSelfSigned: rs.config.AllowSelfSigned,
		}
		return rs.pool.Get(r.Context(), cfg)
	}

	if rs.adtClient == nil {
		return nil, fmt.Errorf("no ADT client configured and no X-SAP-* headers provided")
	}
	return rs.adtClient, nil
}

// Handler returns an http.Handler with all routes registered on a private mux.
// Using a private mux (not http.DefaultServeMux) allows multiple RestServer
// instances and makes the server testable with httptest.NewServer.
func (rs *RestServer) Handler() http.Handler {
	mux := http.NewServeMux()

	// API endpoints for CLI parity (no AI)
	mux.HandleFunc("/api/v1/objects/get", rs.corsHandler(rs.getObjectHandler))
	mux.HandleFunc("/api/v1/objects/create", rs.corsHandler(rs.createObjectHandler))
	mux.HandleFunc("/api/v1/objects/save", rs.corsHandler(rs.saveObjectHandler))
	mux.HandleFunc("/api/v1/objects/search", rs.corsHandler(rs.searchObjectsHandler))
	mux.HandleFunc("/api/v1/objects/list", rs.corsHandler(rs.listObjectsHandler))
	mux.HandleFunc("/api/v1/objects/activate", rs.corsHandler(rs.activateObjectHandler))
	mux.HandleFunc("/api/v1/activate", rs.corsHandler(rs.activateObjectHandler)) // alias used by all external clients
	mux.HandleFunc("/api/v1/service-bindings/publish", rs.corsHandler(rs.publishServiceBindingHandler))
	mux.HandleFunc("/api/v1/packages/contents", rs.corsHandler(rs.packageContentsHandler))
	mux.HandleFunc("/api/v1/syntax-check", rs.corsHandler(rs.syntaxCheckHandler))
	mux.HandleFunc("/api/v1/format", rs.corsHandler(rs.formatSourceHandler))
	mux.HandleFunc("/api/v1/completion", rs.corsHandler(rs.completionHandler))
	mux.HandleFunc("/api/v1/navigation", rs.corsHandler(rs.navigationHandler))
	mux.HandleFunc("/api/v1/unit-tests", rs.corsHandler(rs.unitTestsHandler))
	mux.HandleFunc("/api/v1/transports/info", rs.corsHandler(rs.transportInfoHandler))
	mux.HandleFunc("/api/v1/transports/create", rs.corsHandler(rs.createTransportHandler))
	mux.HandleFunc("/api/v1/system/connect", rs.corsHandler(rs.connectHandler))

	// GitHub proxy endpoints
	mux.HandleFunc("/api/v1/github/oauth/callback", rs.corsHandler(rs.githubOAuthCallbackHandler))
	mux.HandleFunc("/api/v1/github/user", rs.corsHandler(rs.githubUserHandler))
	mux.HandleFunc("/api/v1/github/branches", rs.corsHandler(rs.githubBranchesHandler))
	mux.HandleFunc("/api/v1/github/tree", rs.corsHandler(rs.githubTreeHandler))
	mux.HandleFunc("/api/v1/github/file", rs.corsHandler(rs.githubFileHandler))

	// Removed AI endpoints - return feature removed messages
	mux.HandleFunc("/api/v1/ai/analyze", rs.corsHandler(rs.removedAIHandler))
	mux.HandleFunc("/api/v1/ai/review", rs.corsHandler(rs.removedAIHandler))
	mux.HandleFunc("/api/v1/ai/optimize", rs.corsHandler(rs.removedAIHandler))
	mux.HandleFunc("/api/v1/ai/create", rs.corsHandler(rs.removedAIHandler))

	// Legacy AI endpoints
	mux.HandleFunc("/generate-code", rs.corsHandler(rs.generateCodeHandler))
	mux.HandleFunc("/generate-code-stream", rs.corsHandler(rs.generateCodeStreamHandler))

	// Health and version endpoints
	mux.HandleFunc("/health", rs.healthHandler)
	mux.HandleFunc("/version", rs.versionHandler)

	return mux
}

// Start starts the REST server on the given port.
func (rs *RestServer) Start(port string) {
	rs.logger.Info("Starting REST server with CLI feature parity", zap.String("port", port))
	if err := http.ListenAndServe(":"+port, rs.Handler()); err != nil {
		rs.logger.Fatal("Failed to start server", zap.Error(err))
	}
}

// corsHandler adds CORS headers to responses
func (rs *RestServer) corsHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rs.logger.Debug("Processing request", zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.String("remote_addr", r.RemoteAddr))

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// sendSuccess sends a successful API response
func (rs *RestServer) sendSuccess(w http.ResponseWriter, data any) {
	response := models.APIResponse[any]{
		Success: true,
		Data:    data,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// sendError sends an error API response
func (rs *RestServer) sendError(w http.ResponseWriter, message string, statusCode int) {
	rs.logger.Warn("API error", zap.String("error", message), zap.Int("status", statusCode))

	response := models.APIResponse[any]{
		Success: false,
		Error:   message,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// getObjectHandler handles object retrieval requests (CLI get command equivalent)
func (rs *RestServer) getObjectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.APIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ObjectType == "" || req.ObjectName == "" {
		rs.sendError(w, "object_type and object_name are required", http.StatusBadRequest)
		return
	}

	c, err := rs.clientForRequest(r)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusUnauthorized)
		return
	}

	objectType := strings.ToUpper(req.ObjectType)
	objectName := strings.ToUpper(req.ObjectName)

	rs.logger.Info("Getting object via REST API",
		zap.String("type", objectType),
		zap.String("name", objectName))

	ctx := r.Context()
	var result any

	switch objectType {
	case "PROGRAM", "PROG":
		result, err = c.GetProgram(ctx, objectName)
	case "CLASS", "CLAS":
		result, err = c.GetClass(ctx, objectName)
	case "FUNCTION", "FUNC":
		if len(req.Args) == 0 {
			rs.sendError(w, "function group required in args for function modules", http.StatusBadRequest)
			return
		}
		functionGroup := strings.ToUpper(req.Args[0])
		result, err = c.GetFunction(ctx, objectName, functionGroup)
	case "INCLUDE", "INCL":
		result, err = c.GetInclude(ctx, objectName)
	case "INTERFACE", "INTF":
		result, err = c.GetInterface(ctx, objectName)
	case "STRUCTURE", "STRU":
		result, err = c.GetStructure(ctx, objectName)
	case "TABLE", "TABL":
		result, err = c.GetTable(ctx, objectName)
	case "DDLS", "DATA_DEFINITION":
		result, err = c.GetDDLSource(ctx, objectName)
	case "SRVD", "SERVICE_DEFINITION":
		result, err = c.GetSRVDSource(ctx, objectName)
	case "SRVB", "SERVICE_BINDING":
		result, err = c.GetServiceBinding(ctx, objectName)
	case "FUNCTIONGROUP", "FUGR":
		result, err = c.GetFunctionGroup(ctx, objectName)
	case "DOMAIN", "DOMA":
		result, err = c.GetTypeInfo(ctx, objectName)
	case "DATA_ELEMENT", "DTEL":
		result, err = c.GetTypeInfo(ctx, objectName)
	case "PACKAGE", "PACK":
		result, err = c.GetPackageContents(ctx, objectName)
	default:
		rs.sendError(w, "unsupported object type: "+objectType, http.StatusBadRequest)
		return
	}

	if err != nil {
		rs.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rs.sendSuccess(w, result)
}

// createObjectHandler handles object creation requests (CLI create command equivalent)
func (rs *RestServer) createObjectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.APIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ObjectType == "" || req.ObjectName == "" {
		rs.sendError(w, "object_type and object_name are required", http.StatusBadRequest)
		return
	}

	c, err := rs.clientForRequest(r)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusUnauthorized)
		return
	}

	objectType := strings.ToUpper(req.ObjectType)
	objectName := strings.ToUpper(req.ObjectName)

	ctx := r.Context()

	// abaper-ts convention: source without description means SAVE, not CREATE.
	// editor saveObject() and mcp UpdateObject() both POST here with source+no description.
	// If the object does not yet exist we create a stub first (SE38 create-then-code pattern)
	// so that the very first deploy of a new program also works.
	if req.Source != "" && req.Description == "" {
		pkgForStub := req.Package
		if pkgForStub == "" {
			pkgForStub = "$TMP"
		}

		var saveErr error
		switch objectType {
		case "PROGRAM", "PROG":
			if _, getErr := c.GetProgram(ctx, objectName); getErr != nil {
				if createErr := c.CreateProgram(ctx, objectName, objectName, pkgForStub, ""); createErr != nil {
					rs.sendError(w, createErr.Error(), http.StatusInternalServerError)
					return
				}
			}
			saveErr = c.UpdateProgram(ctx, objectName, req.Source)
		case "CLASS", "CLAS":
			if _, getErr := c.GetClass(ctx, objectName); getErr != nil {
				if createErr := c.CreateClass(ctx, objectName, objectName, pkgForStub, ""); createErr != nil {
					rs.sendError(w, createErr.Error(), http.StatusInternalServerError)
					return
				}
			}
			saveErr = c.UpdateClass(ctx, objectName, req.Source)
		case "INTERFACE", "INTF":
			if _, getErr := c.GetInterface(ctx, objectName); getErr != nil {
				if createErr := c.CreateInterface(ctx, objectName, objectName, ""); createErr != nil {
					rs.sendError(w, createErr.Error(), http.StatusInternalServerError)
					return
				}
			}
			saveErr = c.UpdateInterface(ctx, objectName, req.Source)
		case "INCLUDE", "INCL":
			saveErr = c.UpdateInclude(ctx, objectName, req.Source)
		case "FUNCTION", "FUNC":
			if len(req.Args) == 0 {
				rs.sendError(w, "function group required in args for function modules", http.StatusBadRequest)
				return
			}
			saveErr = c.UpdateFunction(ctx, objectName, strings.ToUpper(req.Args[0]), req.Source)
		case "FUNCTIONGROUP", "FUGR":
			saveErr = c.UpdateFunctionGroup(ctx, objectName, req.Source)
		case "TABLE", "TABL":
			saveErr = c.UpdateTable(ctx, objectName, req.Source)
		case "STRUCTURE", "STRU":
			saveErr = c.UpdateStructure(ctx, objectName, req.Source)
		case "DDLS", "DATA_DEFINITION":
			saveErr = c.UpdateDDLS(ctx, objectName, req.Source)
		case "SRVD", "SERVICE_DEFINITION":
			saveErr = c.UpdateSRVD(ctx, objectName, req.Source)
		default:
			rs.sendError(w, "unsupported object type for save: "+objectType, http.StatusBadRequest)
			return
		}
		if saveErr != nil {
			rs.sendError(w, saveErr.Error(), http.StatusInternalServerError)
			return
		}
		rs.sendSuccess(w, map[string]any{
			"object_name":     objectName,
			"object_type":     objectType,
			"created":         false,
			"source_inserted": true,
			"source_length":   len(req.Source),
			"message":         fmt.Sprintf("%s %s saved successfully", objectType, objectName),
		})
		return
	}

	// Parse creation options from request
	description := req.Description
	if description == "" {
		description = fmt.Sprintf("%s %s", objectType, objectName)
	}

	sourceCode := req.Source
	packageName := req.Package
	if packageName == "" {
		packageName = "$TMP" // Default to local package
	}

	rs.logger.Info("Creating object via REST API",
		zap.String("type", objectType),
		zap.String("name", objectName),
		zap.String("package", packageName),
		zap.Int("source_length", len(sourceCode)))

	switch objectType {
	case "PROGRAM", "PROG":
		err = c.CreateProgram(ctx, objectName, description, packageName, sourceCode)
	case "CLASS", "CLAS":
		err = c.CreateClass(ctx, objectName, description, packageName, sourceCode)
	case "INCLUDE", "INCL":
		err = c.CreateInclude(ctx, objectName, description, sourceCode)
	case "INTERFACE", "INTF":
		err = c.CreateInterface(ctx, objectName, description, sourceCode)
	case "STRUCTURE", "STRU":
		err = c.CreateStructure(ctx, objectName, description, sourceCode)
	case "TABLE", "TABL":
		err = c.CreateTable(ctx, objectName, description, sourceCode)
	case "FUNCTIONGROUP", "FUGR":
		err = c.CreateFunctionGroup(ctx, objectName, description, sourceCode)
	case "FUNCTION", "FUNC":
		if len(req.Args) == 0 {
			rs.sendError(w, "function group required in args for function modules", http.StatusBadRequest)
			return
		}
		err = c.CreateFunction(ctx, objectName, strings.ToUpper(req.Args[0]), description, sourceCode)
	case "DDLS", "DATA_DEFINITION":
		err = c.CreateDDLS(ctx, objectName, description, sourceCode)
	case "SRVD", "SERVICE_DEFINITION":
		err = c.CreateSRVD(ctx, objectName, description, sourceCode)
	case "DOMAIN", "DOMA":
		if req.DomainProperties == nil {
			rs.sendError(w, "domain_properties is required to create a domain", http.StatusBadRequest)
			return
		}
		props := *req.DomainProperties
		if props.Description == "" {
			props.Description = description
		}
		err = c.CreateDomain(ctx, objectName, props)
	case "DATA_ELEMENT", "DTEL":
		if req.DataElementProperties == nil {
			rs.sendError(w, "data_element_properties is required to create a data element", http.StatusBadRequest)
			return
		}
		props := *req.DataElementProperties
		if props.Description == "" {
			props.Description = description
		}
		err = c.CreateDataElement(ctx, objectName, props)
	case "SRVB", "SERVICE_BINDING":
		if req.ServiceBindingProperties == nil {
			rs.sendError(w, "service_binding_properties is required to create a service binding", http.StatusBadRequest)
			return
		}
		props := *req.ServiceBindingProperties
		if props.Description == "" {
			props.Description = description
		}
		err = c.CreateServiceBinding(ctx, objectName, props)
	default:
		rs.sendError(w, "unsupported object type for creation: "+objectType, http.StatusBadRequest)
		return
	}

	if err != nil {
		rs.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := map[string]any{
		"object_name": objectName,
		"object_type": objectType,
		"description": description,
		"package":     packageName,
		"created":     true,
		"timestamp":   time.Now().UTC(),
		"message":     fmt.Sprintf("%s %s created successfully", objectType, objectName),
	}

	if sourceCode != "" {
		result["source_inserted"] = true
		result["source_length"] = len(sourceCode)
		result["activated"] = true
	}

	rs.sendSuccess(w, result)
}

// searchObjectsHandler handles object search requests (CLI search command equivalent)
func (rs *RestServer) searchObjectsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.APIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ObjectName == "" {
		rs.sendError(w, "object_name (search pattern) is required", http.StatusBadRequest)
		return
	}

	c, err := rs.clientForRequest(r)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusUnauthorized)
		return
	}

	pattern := req.ObjectName
	var objectTypes []string
	if req.ObjectType != "" {
		objectTypes = []string{strings.ToUpper(req.ObjectType)}
	}

	rs.logger.Info("Searching objects via REST API",
		zap.String("pattern", pattern),
		zap.Strings("types", objectTypes))

	results, err := c.SearchObjects(r.Context(), pattern, objectTypes)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rs.sendSuccess(w, results)
}

// listObjectsHandler handles object listing requests (CLI list command equivalent).
// Two modes: object_type=packages/package searches package names by pattern;
// otherwise package is required and returns that package's contents, optionally
// filtered by object_type (e.g. "PROG" or "PROG/P").
func (rs *RestServer) listObjectsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.APIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	c, err := rs.clientForRequest(r)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusUnauthorized)
		return
	}

	listType := strings.ToLower(req.ObjectType)

	if listType == "packages" || listType == "package" {
		pattern := req.ObjectName
		if pattern == "" {
			pattern = "*"
		}
		rs.logger.Info("Listing packages via REST API", zap.String("pattern", pattern))
		packages, err := c.ListPackages(r.Context(), pattern)
		if err != nil {
			rs.sendError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rs.sendSuccess(w, packages)
		return
	}

	if req.Package == "" {
		rs.sendError(w, "package is required (or object_type=packages to search package names)", http.StatusBadRequest)
		return
	}

	rs.logger.Info("Listing package contents via REST API",
		zap.String("package", req.Package),
		zap.String("type_filter", req.ObjectType))

	pkg, err := c.GetPackageContents(r.Context(), strings.ToUpper(req.Package))
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	objects := pkg.Objects
	if req.ObjectType != "" {
		filterType := strings.ToUpper(req.ObjectType)
		filtered := make([]types.ADTObject, 0, len(objects))
		for _, obj := range objects {
			objType := strings.ToUpper(obj.Type)
			if objType == filterType || strings.HasPrefix(objType, filterType+"/") {
				filtered = append(filtered, obj)
			}
		}
		objects = filtered
	}

	rs.sendSuccess(w, objects)
}

func (rs *RestServer) packageContentsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req models.APIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	packageName := strings.ToUpper(strings.TrimSpace(req.PackageName))
	if packageName == "" {
		rs.sendError(w, "package_name is required", http.StatusBadRequest)
		return
	}
	c, err := rs.clientForRequest(r)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusUnauthorized)
		return
	}
	result, err := c.GetNodeContents(r.Context(), packageName)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rs.sendSuccess(w, result)
}

func (rs *RestServer) saveObjectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req models.APIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.ObjectType == "" || req.ObjectName == "" {
		rs.sendError(w, "object_type and object_name are required", http.StatusBadRequest)
		return
	}
	c, err := rs.clientForRequest(r)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusUnauthorized)
		return
	}
	objectType := strings.ToUpper(req.ObjectType)
	objectName := strings.ToUpper(req.ObjectName)
	ctx := r.Context()

	// DDIC property objects (domain, data element) are updated via structured
	// properties, not source text — handle them before the source requirement.
	switch objectType {
	case "DOMAIN", "DOMA":
		if req.DomainProperties == nil {
			rs.sendError(w, "domain_properties is required to update a domain", http.StatusBadRequest)
			return
		}
		if err := c.UpdateDomain(ctx, objectName, *req.DomainProperties); err != nil {
			rs.sendError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rs.sendSuccess(w, map[string]any{"object_name": objectName, "object_type": objectType, "message": fmt.Sprintf("%s %s saved successfully", objectType, objectName)})
		return
	case "DATA_ELEMENT", "DTEL":
		if req.DataElementProperties == nil {
			rs.sendError(w, "data_element_properties is required to update a data element", http.StatusBadRequest)
			return
		}
		if err := c.UpdateDataElement(ctx, objectName, *req.DataElementProperties); err != nil {
			rs.sendError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rs.sendSuccess(w, map[string]any{"object_name": objectName, "object_type": objectType, "message": fmt.Sprintf("%s %s saved successfully", objectType, objectName)})
		return
	case "SRVB", "SERVICE_BINDING":
		if req.ServiceBindingProperties == nil {
			rs.sendError(w, "service_binding_properties is required to update a service binding", http.StatusBadRequest)
			return
		}
		if err := c.UpdateServiceBinding(ctx, objectName, *req.ServiceBindingProperties); err != nil {
			rs.sendError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rs.sendSuccess(w, map[string]any{"object_name": objectName, "object_type": objectType, "message": fmt.Sprintf("%s %s saved successfully", objectType, objectName)})
		return
	}

	if req.Source == "" {
		rs.sendError(w, "source is required", http.StatusBadRequest)
		return
	}
	switch objectType {
	case "PROGRAM", "PROG":
		err = c.UpdateProgram(ctx, objectName, req.Source)
	case "CLASS", "CLAS":
		err = c.UpdateClass(ctx, objectName, req.Source)
	case "INCLUDE", "INCL":
		err = c.UpdateInclude(ctx, objectName, req.Source)
	case "INTERFACE", "INTF":
		err = c.UpdateInterface(ctx, objectName, req.Source)
	case "FUNCTION", "FUNC":
		if len(req.Args) == 0 {
			rs.sendError(w, "function group required in args for function modules", http.StatusBadRequest)
			return
		}
		err = c.UpdateFunction(ctx, objectName, strings.ToUpper(req.Args[0]), req.Source)
	case "FUNCTIONGROUP", "FUGR":
		err = c.UpdateFunctionGroup(ctx, objectName, req.Source)
	case "TABLE", "TABL":
		err = c.UpdateTable(ctx, objectName, req.Source)
	case "STRUCTURE", "STRU":
		err = c.UpdateStructure(ctx, objectName, req.Source)
	case "DDLS", "DATA_DEFINITION":
		err = c.UpdateDDLS(ctx, objectName, req.Source)
	case "SRVD", "SERVICE_DEFINITION":
		err = c.UpdateSRVD(ctx, objectName, req.Source)
	default:
		rs.sendError(w, "unsupported object type for save: "+objectType, http.StatusBadRequest)
		return
	}
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rs.sendSuccess(w, map[string]any{
		"object_name":     objectName,
		"object_type":     objectType,
		"source_inserted": true,
		"source_length":   len(req.Source),
		"message":         fmt.Sprintf("%s %s saved successfully", objectType, objectName),
	})
}

func (rs *RestServer) activateObjectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req models.APIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.ObjectType == "" || req.ObjectName == "" {
		rs.sendError(w, "object_type and object_name are required", http.StatusBadRequest)
		return
	}
	c, err := rs.clientForRequest(r)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusUnauthorized)
		return
	}
	objectType := strings.ToUpper(req.ObjectType)
	objectName := strings.ToUpper(req.ObjectName)
	ctx := r.Context()
	// Optionally save source before activating.
	if req.Source != "" {
		switch objectType {
		case "PROGRAM", "PROG":
			err = c.UpdateProgram(ctx, objectName, req.Source)
		case "CLASS", "CLAS":
			err = c.UpdateClass(ctx, objectName, req.Source)
		case "INCLUDE", "INCL":
			err = c.UpdateInclude(ctx, objectName, req.Source)
		case "INTERFACE", "INTF":
			err = c.UpdateInterface(ctx, objectName, req.Source)
		}
		if err != nil {
			rs.sendError(w, "save before activate failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	result, err := c.ActivateObject(ctx, objectType, objectName)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Shape matches abaper-ts activate.ts: data.activated (bool) instead of data.success.
	rs.sendSuccess(w, map[string]any{
		"object_name": result.ObjectName,
		"object_type": result.ObjectType,
		"activated":   result.Success,
		"messages":    result.Messages,
	})
}

// publishServiceBindingHandler registers a Service Binding's OData service
// at runtime. The binding must already be created and active.
func (rs *RestServer) publishServiceBindingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req models.APIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.ObjectName == "" {
		rs.sendError(w, "object_name is required", http.StatusBadRequest)
		return
	}
	c, err := rs.clientForRequest(r)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusUnauthorized)
		return
	}
	objectName := strings.ToUpper(req.ObjectName)
	result, err := c.PublishServiceBinding(r.Context(), objectName)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rs.sendSuccess(w, result)
}

func (rs *RestServer) syntaxCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req models.APIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.ObjectType == "" || req.ObjectName == "" || req.Source == "" {
		rs.sendError(w, "object_type, object_name, and source are required", http.StatusBadRequest)
		return
	}
	c, err := rs.clientForRequest(r)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusUnauthorized)
		return
	}
	result, err := c.SyntaxCheck(r.Context(), strings.ToUpper(req.ObjectType), strings.ToUpper(req.ObjectName), req.Source)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rs.sendSuccess(w, result)
}

func (rs *RestServer) formatSourceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req models.APIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Source == "" {
		rs.sendError(w, "source is required", http.StatusBadRequest)
		return
	}
	c, err := rs.clientForRequest(r)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusUnauthorized)
		return
	}
	formatted, err := c.FormatSource(r.Context(), req.Source)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rs.sendSuccess(w, map[string]string{"source": formatted})
}

func (rs *RestServer) completionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req models.APIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.ObjectType == "" || req.ObjectName == "" || req.Source == "" {
		rs.sendError(w, "object_type, object_name, source, line, and column are required", http.StatusBadRequest)
		return
	}
	c, err := rs.clientForRequest(r)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusUnauthorized)
		return
	}
	proposals, err := c.GetCompletionProposals(r.Context(), strings.ToUpper(req.ObjectType), strings.ToUpper(req.ObjectName), req.Source, req.Line, req.Column)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rs.sendSuccess(w, proposals)
}

func (rs *RestServer) navigationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req models.APIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.ObjectType == "" || req.ObjectName == "" || req.Source == "" {
		rs.sendError(w, "object_type, object_name, source, line, and column are required", http.StatusBadRequest)
		return
	}
	c, err := rs.clientForRequest(r)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusUnauthorized)
		return
	}
	target, err := c.GetNavigationTarget(r.Context(), strings.ToUpper(req.ObjectType), strings.ToUpper(req.ObjectName), req.Source, req.Line, req.Column)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rs.sendSuccess(w, target)
}

func (rs *RestServer) unitTestsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req models.APIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.ObjectType == "" || req.ObjectName == "" {
		rs.sendError(w, "object_type and object_name are required", http.StatusBadRequest)
		return
	}
	c, err := rs.clientForRequest(r)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusUnauthorized)
		return
	}
	result, err := c.RunUnitTests(r.Context(), strings.ToUpper(req.ObjectType), strings.ToUpper(req.ObjectName))
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rs.sendSuccess(w, result)
}

func (rs *RestServer) transportInfoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req models.APIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.ObjectType == "" || req.ObjectName == "" {
		rs.sendError(w, "object_type and object_name are required", http.StatusBadRequest)
		return
	}
	c, err := rs.clientForRequest(r)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusUnauthorized)
		return
	}
	result, err := c.GetTransportInfo(r.Context(), strings.ToUpper(req.ObjectType), strings.ToUpper(req.ObjectName))
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rs.sendSuccess(w, result)
}

func (rs *RestServer) createTransportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req models.APIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.ObjectType == "" || req.ObjectName == "" || req.Description == "" {
		rs.sendError(w, "object_type, object_name, and description are required", http.StatusBadRequest)
		return
	}
	c, err := rs.clientForRequest(r)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusUnauthorized)
		return
	}
	transportNumber, err := c.CreateTransport(r.Context(), strings.ToUpper(req.ObjectType), strings.ToUpper(req.ObjectName), req.Description, strings.ToUpper(req.Package))
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rs.sendSuccess(w, map[string]string{
		"transport_number": transportNumber,
		"description":      req.Description,
		"package":          strings.ToUpper(req.Package),
	})
}

// connectHandler handles connection test requests (CLI connect command equivalent)
func (rs *RestServer) connectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		rs.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rs.logger.Info("Testing ADT connection via REST API")

	c, err := rs.clientForRequest(r)
	if err != nil {
		rs.sendError(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err := c.TestConnection(); err != nil {
		rs.logger.Error("ADT connection test failed", zap.Error(err))
		rs.sendError(w, "Connection failed: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	connectionStatus := map[string]any{
		"status":        "connected",
		"authenticated": c.IsAuthenticated(),
		"timestamp":     time.Now().UTC(),
		"message":       "ADT connection successful",
	}

	rs.sendSuccess(w, connectionStatus)
}

// healthHandler handles health check requests
func (rs *RestServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	adtStatus := "disconnected"
	if rs.adtClient != nil && rs.adtClient.IsAuthenticated() {
		adtStatus = "connected"
	}

	poolSize := 0
	if rs.pool != nil {
		poolSize = rs.pool.Size()
		if poolSize > 0 {
			adtStatus = "connected"
		}
	}

	health := map[string]any{
		"status":           "healthy",
		"timestamp":        time.Now().UTC(),
		"adt_status":       adtStatus,
		"pool_connections": poolSize,
		"features": map[string]bool{
			"quiet_mode_default": true,
			"file_logging":       true,
			"adt_integration":    true,
			"cli_parity":         true,
			"ai_removed":         true,
			"connection_pool":    rs.pool != nil,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// versionHandler handles version requests
func (rs *RestServer) versionHandler(w http.ResponseWriter, r *http.Request) {
	version := map[string]string{
		"features": "CLI and REST Services (No AI)",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(version)
}

// Removed legacy AI handlers - now returns appropriate messages
func (rs *RestServer) generateCodeHandler(w http.ResponseWriter, r *http.Request) {
	rs.sendError(w, "AI code generation features have been removed. This endpoint is no longer available.", http.StatusGone)
}

func (rs *RestServer) generateCodeStreamHandler(w http.ResponseWriter, r *http.Request) {
	rs.sendError(w, "AI streaming features have been removed. This endpoint is no longer available.", http.StatusGone)
}

// removedAIHandler handles requests to removed AI endpoints
func (rs *RestServer) removedAIHandler(w http.ResponseWriter, r *http.Request) {
	rs.sendError(w, "AI features have been removed from this version. This endpoint is no longer available.", http.StatusGone)
}
