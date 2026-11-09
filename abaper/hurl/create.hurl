# 🔐 Authentication & Discovery
# Complete collection of ABAP Development Tools (ADT) authentication and discovery endpoints
# Environment variables: host, username, password, client

# Login Compatibility Check - Initial login and CSRF token fetch. Run this first!
GET {{host}}/sap/bc/adt/compatibility/graph
Accept: application/xml
x-csrf-token: fetch
[QueryStringParams]
sap-client: 001
sap-language: EN
[BasicAuth]
{{username}}: {{password}}

HTTP 200
[Captures]
csrf_token: header "x-csrf-token"

# Create Program
POST {{host}}/sap/bc/adt/programs/programs/ztest_metadata_only?_action=LOCK&accessMode=MODIFY
Accept: application/*
X-CSRF-Token: {{csrf_token}}
[QueryStringParams]
sap-client: 001
sap-language: EN
[BasicAuth]
{{username}}: {{password}}

HTTP 200
# Capture LOCK_HANDLE from the XML response
[Captures]
lockHandle: xpath "string(//LOCK_HANDLE)"

# Update Program Source Code
PUT {{host}}/sap/bc/adt/programs/programs/ztest_metadata_only/source/main?lockHandle={{lockHandle}}
Content-Type: text/plain; charset=utf-8
X-CSRF-Token: {{csrf_token}}
[QueryStringParams]
sap-client: 001
sap-language: EN
[BasicAuth]
{{username}}: {{password}}

```
REPORT z_test_program.
WRITE: 'Hello from ADT API!'.
```

HTTP 200

# Unlock program at the end
POST {{host}}/sap/bc/adt/programs/programs/ztest_metadata_only?_action=UNLOCK
Accept: application/*
X-CSRF-Token: {{csrf_token}}
[QueryStringParams]
sap-client: 001
sap-language: EN
[BasicAuth]
{{username}}: {{password}}

<asx:abap xmlns:asx="http://www.sap.com/abapxml" version="1.0">
  <asx:values>
    <DATA>
      <LOCK_HANDLE>{{lockHandle}}</LOCK_HANDLE>
    </DATA>
  </asx:values>
</asx:abap>

HTTP 200
