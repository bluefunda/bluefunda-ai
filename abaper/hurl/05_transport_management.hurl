# 🚚 Transport Management
# Transport request creation and management
# Environment variables: SAP_HOST, SAP_USERNAME, SAP_PASSWORD, SAP_CLIENT

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
