# ⚡ Activation
# Object activation and inactive object management
# Environment variables: SAP_HOST, SAP_USERNAME, SAP_PASSWORD, SAP_CLIENT

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
