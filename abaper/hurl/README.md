# ABAP ADT API Hurl Scripts

This directory contains Hurl scripts for all ABAP Development Tools (ADT) REST API endpoints, converted from the Postman collection.

## Overview

The collection contains **111 endpoints** organized into **12 categories** covering all major ADT functionalities:

1. **🔐 Authentication & Discovery** - Login, CSRF token, service discovery
2. **📂 Repository Management** - Object types, search, repository navigation
3. **📄 Object Management** - Object CRUD operations, locking, source management
4. **⚡ Activation** - Object activation and inactive object management
5. **🚚 Transport Management** - Transport requests and system management
6. **💻 Code Development** - Syntax check, code completion, pretty print
7. **🧪 Testing** - Unit tests and ABAP class execution
8. **📊 Data & Services** - Data preview and SQL query execution
9. **🐞 Debugger** - Debugging functionality, breakpoints, stack traces
10. **🔍 ATC & Quality** - ABAP Test Cockpit and code quality checks
11. **📊 Traces & Performance** - Runtime traces and performance analysis
12. **📋 Feeds & Runtime** - Feeds and runtime information

## Environment Variables

Before running any scripts, set these environment variables:

```bash
export SAP_HOST="https://your-sap-server:8000"
export SAP_USERNAME="your_username"
export SAP_PASSWORD="your_password"
export SAP_CLIENT="100"
```

### Example Setup

```bash
# For local SAP NetWeaver system
export SAP_HOST="https://vhcalnplci.local:8000"
export SAP_USERNAME="DEVELOPER"
export SAP_PASSWORD="your_password"
export SAP_CLIENT="100"

# For cloud SAP system
export SAP_HOST="https://your-tenant.sap.com"
export SAP_USERNAME="your_email@company.com"
export SAP_PASSWORD="your_password"
export SAP_CLIENT="100"
```

## File Structure

```
hurl/
├── README.md                          # This file
├── all_endpoints.hurl                 # All endpoints in one file
├── 01_authentication_discovery.hurl   # Authentication & Discovery
├── 02_repository_management.hurl      # Repository Management
├── 03_object_management.hurl          # Object Management
├── 04_activation.hurl                 # Activation
├── 05_transport_management.hurl       # Transport Management
├── 06_code_development.hurl           # Code Development
├── 07_testing.hurl                    # Testing
├── 08_data_services.hurl              # Data & Services
├── 09_debugger.hurl                   # Debugger
├── 10_atc_quality.hurl                # ATC & Quality
├── 11_traces_performance.hurl         # Traces & Performance
└── 12_feeds_runtime.hurl              # Feeds & Runtime
```

## Usage Examples

### 1. Setup and Run Authentication First

First, ensure your environment variables are set, then run authentication:

```bash
# Check environment
./hurl/run_tests.sh check

# Run authentication (automatically generates hurl.config)
./hurl/run_tests.sh auth

# Or manually with variables file
hurl --variables-file hurl.config hurl/01_authentication_discovery.hurl
```

### 2. Run Individual Categories

```bash
# Using the test runner (recommended)
./hurl/run_tests.sh repo
./hurl/run_tests.sh objects
./hurl/run_tests.sh testing

# Or manually with variables file
hurl --variables-file hurl.config hurl/02_repository_management.hurl
hurl --variables-file hurl.config hurl/03_object_management.hurl
hurl --variables-file hurl.config hurl/07_testing.hurl
```

### 3. Run All Endpoints

```bash
# Using the test runner
./hurl/run_tests.sh all

# Or manually
hurl --variables-file hurl.config hurl/all_endpoints.hurl
```

### 4. Generate Configuration File

The hurl.config file is automatically generated from environment variables:

```bash
# Generate config manually
./hurl/generate_config.sh

# Or it's auto-generated when using run_tests.sh
./hurl/run_tests.sh auth

# Manual configuration
cp hurl.config.example hurl.config
# Edit hurl.config with your values
```

### 5. Run with Output Options

```bash
# Save response to file
hurl --variables-file hurl.config --output response.xml hurl/01_authentication_discovery.hurl

# Verbose output
hurl --variables-file hurl.config --verbose hurl/01_authentication_discovery.hurl

# JSON output
hurl --variables-file hurl.config --json hurl/01_authentication_discovery.hurl

# Test mode (just check HTTP status)
hurl --variables-file hurl.config --test hurl/01_authentication_discovery.hurl
```

## Important Notes

### Authentication Flow

1. **Always run authentication first** - The login compatibility check must be executed first to obtain the CSRF token
2. **CSRF Token** - Many operations require a valid CSRF token obtained from the login request
3. **Basic Auth** - All requests use HTTP Basic Authentication with SAP credentials

### Object Locking

When working with objects:

1. **Lock** the object before modification
2. **Modify** the object source/metadata
3. **Unlock** the object after changes
4. **Activate** the object to make changes effective

Example workflow:
```bash
# 1. Lock object
hurl hurl/03_object_management.hurl --test "Lock Object"

# 2. Update source
hurl hurl/03_object_management.hurl --test "Update Object Source"

# 3. Unlock object
hurl hurl/03_object_management.hurl --test "Unlock Object"

# 4. Activate object
hurl hurl/04_activation.hurl --test "Activate Objects"
```

### Customization

You can customize the scripts by:

1. **Modifying object names** - Change `z_test_program`, `ZCL_TEST_CLASS` etc. to your objects
2. **Adjusting query parameters** - Modify search criteria, row limits, etc.
3. **Changing request bodies** - Update XML/ABAP code content as needed

### Error Handling

Common HTTP status codes:
- **200** - Success
- **201** - Created (for object creation)
- **401** - Authentication failed
- **403** - Forbidden (check CSRF token)
- **404** - Object/endpoint not found
- **500** - Server error

## Integration with abaper Project

These Hurl scripts are designed to work with the `abaper` project and can be used to:

1. **Test ADT API endpoints** before implementing in Go
2. **Validate API responses** and understand expected formats
3. **Debug authentication issues** 
4. **Prototype new features** quickly
5. **Create integration tests** for the REST server

## Example Workflows

### Creating a New Program

```bash
# 1. Authenticate
hurl hurl/01_authentication_discovery.hurl

# 2. Create program
hurl hurl/03_object_management.hurl --test "Create Program"

# 3. Lock program
hurl hurl/03_object_management.hurl --test "Lock Object"

# 4. Update source
hurl hurl/03_object_management.hurl --test "Update Object Source"

# 5. Unlock program
hurl hurl/03_object_management.hurl --test "Unlock Object"

# 6. Activate program
hurl hurl/04_activation.hurl --test "Activate Objects"
```

### Running Quality Checks

```bash
# 1. Authenticate
hurl hurl/01_authentication_discovery.hurl

# 2. Syntax check
hurl hurl/06_code_development.hurl --test "Syntax Check"

# 3. ATC check
hurl hurl/10_atc_quality.hurl --test "Create ATC Run"

# 4. Unit tests
hurl hurl/07_testing.hurl --test "Run Unit Tests"
```

## References

- [Hurl Documentation](https://hurl.dev/)
- [ABAP ADT API Documentation](https://help.sap.com/docs/ABAP_PLATFORM/c238d694b825421f940829321ffa326a/289c4bb89e604db8b8ef52c1c8dd7bec.html)
- [Original Postman Collection](../postman/ABAP%20ADT%20API%20Collection.postman_collection.json)
- [abap-adt-api TypeScript Reference](../../abap-adt-api/)
