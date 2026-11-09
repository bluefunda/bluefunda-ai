# ABAP ADT API Collection - Complete Hurl Scripts
# Complete collection of ABAP Development Tools (ADT) REST API endpoints
# Converted from Postman collection to Hurl format
# 
# Environment variables required:
# - SAP_HOST: SAP server URL (e.g., https://vhcalnplci.local:8000)
# - SAP_USERNAME: SAP username
# - SAP_PASSWORD: SAP password  
# - SAP_CLIENT: SAP client number (e.g., 100)

#################################################################
# 🔐 Authentication & Discovery
#################################################################

# Login Compatibility Check - Initial login and CSRF token fetch. Run this first!
GET {{SAP_HOST}}/sap/bc/adt/compatibility/graph
Accept: application/xml
x-csrf-token: fetch
[QueryStringParams]
sap-client: {{SAP_CLIENT}}
sap-language: EN
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200
[Captures]
csrf_token: header "x-csrf-token"

###

# ADT Discovery - Discover available ADT services
GET {{SAP_HOST}}/sap/bc/adt/discovery
Accept: application/atomsvc+xml
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# ADT Core Discovery - Discover ADT core services
GET {{SAP_HOST}}/sap/bc/adt/core/discovery
Accept: application/atomsvc+xml
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Get Reentrance Ticket - Get reentrance ticket for debugging
GET {{SAP_HOST}}/sap/bc/adt/security/reentranceticket
Accept: text/plain
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Logout - Logout and clear session
GET {{SAP_HOST}}/sap/public/bc/icf/logoff
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

#################################################################
# 📂 Repository Management
#################################################################

# Get Object Types - Get available object types
GET {{SAP_HOST}}/sap/bc/adt/repository/informationsystem/objecttypes
Accept: application/xml
[QueryStringParams]
maxItemCount: 999
name: *
data: usedByProvider
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Search Objects - Search for objects with wildcard
GET {{SAP_HOST}}/sap/bc/adt/repository/informationsystem/search
Accept: application/*
[QueryStringParams]
operation: quickSearch
query: Z*
maxResults: 100
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Find Object Path - Find path for specific object
POST {{SAP_HOST}}/sap/bc/adt/repository/nodepath
Accept: application/xml
[QueryStringParams]
uri: /sap/bc/adt/programs/programs/z_test
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Node Contents - Get contents of a repository node
POST {{SAP_HOST}}/sap/bc/adt/repository/informationsystem/nodecontents
Accept: application/xml
[QueryStringParams]
parent_type: devc
parent_name: $TMP
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

#################################################################
# 📄 Object Management
#################################################################

# Get Object Structure - Get object structure metadata
GET {{SAP_HOST}}/sap/bc/adt/programs/programs/z_test_program
Accept: application/*
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Get Object Source - Get source code of an object
GET {{SAP_HOST}}/sap/bc/adt/programs/programs/z_test_program/source/main
Accept: text/plain
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Lock Object - Lock object for modification
POST {{SAP_HOST}}/sap/bc/adt/programs/programs/ztest1
Accept: application/*
X-CSRF-Token: {{csrf_token}}
[QueryStringParams]
_action: LOCK
accessMode: MODIFY
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200
[Captures]
lock_handle: xpath "//LOCK_HANDLE/text()"

###

# Update Object Source - Update source code of an object
PUT {{SAP_HOST}}/sap/bc/adt/programs/programs/z_test_program/source/main
Content-Type: text/plain; charset=utf-8
[QueryStringParams]
lockHandle: {{lock_handle}}
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
REPORT z_test_program.

WRITE: 'Hello from ADT API!'.
```

HTTP 200

###

# Unlock Object - Unlock object after modification
POST {{SAP_HOST}}/sap/bc/adt/programs/programs/z_test_program
[QueryStringParams]
_action: UNLOCK
lockHandle: {{lock_handle}}
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Create Program - Create a new ABAP program
POST {{SAP_HOST}}/sap/bc/adt/programs/programs
Content-Type: application/*
x-csrf-token: {{csrf_token}}
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
<?xml version="1.0" encoding="UTF-8"?>
<program:abapProgram xmlns:program="http://www.sap.com/adt/programs/programs"
  xmlns:adtcore="http://www.sap.com/adt/core"
  adtcore:description="My Test Program"
  adtcore:name="Z_MY_TEST_PROGRAM" 
  adtcore:type="PROG/P"
  adtcore:responsible="DEVELOPER">
  <adtcore:packageRef adtcore:name="ZTEST"/>
</program:abapProgram>
```

HTTP 201

###

# Create Class - Create a new ABAP class
POST {{SAP_HOST}}/sap/bc/adt/oo/classes
Content-Type: application/*
x-csrf-token: {{csrf_token}}
[QueryStringParams]
corrNr: A4HK900120
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
<?xml version="1.0" encoding="UTF-8"?>
<class:abapClass xmlns:class="http://www.sap.com/adt/oo/classes"
  xmlns:adtcore="http://www.sap.com/adt/core"
  adtcore:description="My Test Class"
  adtcore:name="ZCL_MY_TEST_CLASS" 
  adtcore:type="CLAS/OC"
  adtcore:responsible="PHANIKUMARPS">
  <adtcore:packageRef adtcore:name="ZBDA"/>
</class:abapClass>
```

HTTP 201

###

#################################################################
# ⚡ Activation
#################################################################

# Activate Objects - Activate one or more objects
POST {{SAP_HOST}}/sap/bc/adt/activation
Content-Type: application/*
[QueryStringParams]
method: activate
preauditRequested: true
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
<?xml version="1.0"?>
<adtcore:objectReferences xmlns:adtcore="http://www.sap.com/adt/core">
<adtcore:objectReference adtcore:uri="/sap/bc/adt/programs/programs/z_test_program" adtcore:name="Z_TEST_PROGRAM"/>
</adtcore:objectReferences>
```

HTTP 200

###

# Get Inactive Objects - Get list of inactive objects for current user
GET {{SAP_HOST}}/sap/bc/adt/activation/inactiveobjects
Accept: application/vnd.sap.adt.inactivectsobjects.v1+xml
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

#################################################################
# 🚚 Transport Management
#################################################################

# Transport Info Check - Check transport requirements for object
POST {{SAP_HOST}}/sap/bc/adt/cts/transportchecks
Content-Type: application/vnd.sap.as+xml
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
<?xml version="1.0"?>
<asx:abap xmlns:asx="http://www.sap.com/abapxml" version="1.0">
<asx:values><DATA><URI>/sap/bc/adt/programs/programs/z_test</URI><DEVCLASS>$TMP</DEVCLASS><OPERATION>I</OPERATION></DATA></asx:values>
</asx:abap>
```

HTTP 200

###

# Get Transport Requests - Get transport requests for user
GET {{SAP_HOST}}/sap/bc/adt/cts/transportrequests
[QueryStringParams]
user: {{SAP_USERNAME}}
targets: true
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Create Transport - Create new transport request
POST {{SAP_HOST}}/sap/bc/adt/cts/transports
Content-Type: application/vnd.sap.as+xml
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
<?xml version="1.0"?>
<asx:abap xmlns:asx="http://www.sap.com/abapxml" version="1.0">
<asx:values><DATA><REF>/sap/bc/adt/programs/programs/z_test</REF><REQUEST_TEXT>Test Transport</REQUEST_TEXT><DEVCLASS>ZTEST</DEVCLASS><OPERATION>I</OPERATION></DATA></asx:values>
</asx:abap>
```

HTTP 200

###

# Get System Users - Get list of system users
GET {{SAP_HOST}}/sap/bc/adt/system/users
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

#################################################################
# 💻 Code Development
#################################################################

# Syntax Check - Check ABAP code syntax
POST {{SAP_HOST}}/sap/bc/adt/checkruns
Content-Type: text/plain
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
REPORT z_test.
WRITE: 'Hello World'.
```

HTTP 200

###

# Code Completion - Get code completion suggestions
POST {{SAP_HOST}}/sap/bc/adt/abap/completion/semantics
Content-Type: text/plain
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
REPORT z_test.
WR
```

HTTP 200

###

# Pretty Print - Format ABAP code
POST {{SAP_HOST}}/sap/bc/adt/abap/formatter
Content-Type: text/plain
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
report z_test.
write:'hello'.
```

HTTP 200

###

#################################################################
# 🧪 Testing
#################################################################

# Run Unit Tests - Execute unit tests for specified objects
POST {{SAP_HOST}}/sap/bc/adt/abapunit/testruns
Content-Type: application/vnd.sap.adt.abapunit.testruns.v4+xml
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
<?xml version="1.0"?>
<aunit:runConfiguration xmlns:aunit="http://www.sap.com/adt/aunit">
<aunit:external><aunit:coverage active="true"/></aunit:external>
<aunit:options><aunit:uriType value="semantic"/></aunit:options>
<aunit:objectSets><aunit:objectSet kind="inclusive">
<adtcore:objectReferences xmlns:adtcore="http://www.sap.com/adt/core">
<adtcore:objectReference adtcore:uri="/sap/bc/adt/oo/classes/zcl_test"/>
</adtcore:objectReferences></aunit:objectSet></aunit:objectSets>
</aunit:runConfiguration>
```

HTTP 200

###

# Run ABAP Class - Execute ABAP class (if_oo_adt_classrun)
POST {{SAP_HOST}}/sap/bc/adt/oo/classrun/ZCL_TEST_CLASS
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

#################################################################
# 📊 Data & Services
#################################################################

# Table Contents - Preview table data
POST {{SAP_HOST}}/sap/bc/adt/datapreview/ddic
Content-Type: text/plain
x-csrf-token: {{csrf_token}}
[QueryStringParams]
rowNumber: 100
ddicEntityName: MARA
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Run SQL Query - Execute freestyle SQL query
POST {{SAP_HOST}}/sap/bc/adt/datapreview/freestyle
Content-Type: text/plain
x-csrf-token: {{csrf_token}}
[QueryStringParams]
rowNumber: 100
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
SELECT * FROM MARA WHERE MATNR LIKE 'T%'
```

HTTP 200

###

#################################################################
# 🐞 Debugger
#################################################################

# Check Debug Listeners - Check active debug listeners
GET {{SAP_HOST}}/sap/bc/adt/debugger/listeners
[QueryStringParams]
debuggingMode: user
requestUser: {{SAP_USERNAME}}
terminalId: 12345
ideId: 67890
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Set Breakpoint - Set line breakpoint in code
POST {{SAP_HOST}}/sap/bc/adt/debugger/breakpoints
Content-Type: application/xml
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
<?xml version="1.0"?>
<dbg:breakpoints xmlns:dbg="http://www.sap.com/adt/debugger" scope="external" debuggingMode="user" requestUser="{{SAP_USERNAME}}" terminalId="12345" ideId="67890">
<syncScope mode="full"/>
<breakpoint kind="line" clientId="test" adtcore:uri="/sap/bc/adt/programs/programs/z_test/source/main#start=5" xmlns:adtcore="http://www.sap.com/adt/core"/>
</dbg:breakpoints>
```

HTTP 200

###

# Get Debug Stack - Get current debug stack trace
GET {{SAP_HOST}}/sap/bc/adt/debugger/stack
[QueryStringParams]
method: getStack
semanticURIs: true
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

#################################################################
# 🔍 ATC & Quality
#################################################################

# ATC Customizing - Get ATC customizing settings
GET {{SAP_HOST}}/sap/bc/adt/atc/customizing
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Create ATC Run - Create and execute ATC check run
POST {{SAP_HOST}}/sap/bc/adt/atc/runs
Content-Type: application/xml
[QueryStringParams]
worklistId: DEFAULT
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
<?xml version="1.0"?>
<atc:run xmlns:atc="http://www.sap.com/adt/atc" maximumVerdicts="100">
<objectSets xmlns:adtcore="http://www.sap.com/adt/core">
<objectSet kind="inclusive">
<adtcore:objectReferences>
<adtcore:objectReference adtcore:uri="/sap/bc/adt/programs/programs/z_test"/>
</adtcore:objectReferences>
</objectSet>
</objectSets>
</atc:run>
```

HTTP 200

###

# Get ATC Worklist - Get ATC worklist results
GET {{SAP_HOST}}/sap/bc/adt/atc/worklists/12345
Accept: application/atc.worklist.v1+xml
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

#################################################################
# 📊 Traces & Performance
#################################################################

# List Traces - Get list of runtime traces for user
GET {{SAP_HOST}}/sap/bc/adt/runtime/traces/abaptraces
[QueryStringParams]
user: {{SAP_USERNAME}}
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Get Trace Hit List - Get hit list for specific trace
GET {{SAP_HOST}}/sap/bc/adt/runtime/traces/abaptraces/12345/hitlist
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Set Trace Parameters - Configure trace parameters
POST {{SAP_HOST}}/sap/bc/adt/runtime/traces/abaptraces/parameters
Content-Type: application/xml
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
<?xml version="1.0"?>
<trc:parameters xmlns:trc="http://www.sap.com/adt/runtime/traces/abaptraces">
<trc:allMiscAbapStatements value="true"/>
<trc:description value="Test Trace"/>
<trc:maxSizeForTraceFile value="1000"/>
</trc:parameters>
```

HTTP 200

###

#################################################################
# 📋 Feeds & Runtime
#################################################################

# Get Feeds - Get available feeds
GET {{SAP_HOST}}/sap/bc/adt/feeds
Accept: application/atom+xml;type=feed
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Get Dumps Feed - Get runtime dumps feed
GET {{SAP_HOST}}/sap/bc/adt/runtime/dumps
Accept: application/atom+xml;type=feed
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200
