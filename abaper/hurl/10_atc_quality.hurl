# 🔍 ATC & Quality
# ABAP Test Cockpit (ATC) and code quality checks
# Environment variables: SAP_HOST, SAP_USERNAME, SAP_PASSWORD, SAP_CLIENT

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
