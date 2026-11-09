# 🆕 Create NEW Program (not modify existing)
# This creates a completely new program instead of modifying existing one

# 1️⃣ Get CSRF Token
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

# 2️⃣ Create NEW Program (POST to create, not modify)
POST {{host}}/sap/bc/adt/programs/programs
Accept: application/xml
Content-Type: application/xml
X-CSRF-Token: {{csrf_token}}
[QueryStringParams]
sap-client: 001
sap-language: EN
[BasicAuth]
{{username}}: {{password}}
```
<?xml version="1.0" encoding="UTF-8"?>
<program:abapProgram xmlns:program="http://www.sap.com/adt/programs" xmlns:adtcore="http://www.sap.com/adt/core" adtcore:type="PROG/P" adtcore:description="Test program created via Hurl" adtcore:name="ZTEST_HURL_NEW" adtcore:masterLanguage="EN">
  <adtcore:packageRef adtcore:name="$TMP"/>
</program:abapProgram>
```

HTTP 201
[Captures]
program_uri: header "location"

# 3️⃣ Now get source of the new program and update it
PUT {{host}}/sap/bc/adt/programs/programs/ztest_hurl_new/source/main
Content-Type: text/plain; charset=utf-8
X-CSRF-Token: {{csrf_token}}
[QueryStringParams]
sap-client: 001
sap-language: EN
[BasicAuth]
{{username}}: {{password}}
```
REPORT ztest_hurl_new.
WRITE: 'Hello from NEW Hurl program!'.
WRITE: / 'Created at: ' && sy-datum && ' ' && sy-uzeit.
```

HTTP 200
