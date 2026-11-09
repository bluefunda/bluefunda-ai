# 📊 Traces & Performance
# Runtime traces and performance analysis
# Environment variables: SAP_HOST, SAP_USERNAME, SAP_PASSWORD, SAP_CLIENT

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
