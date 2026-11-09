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

# Activate Program
POST {{host}}/sap/bc/adt/activation
Accept: application/*
Content-Type: application/*
X-CSRF-Token: {{csrf_token}}
[QueryStringParams]
method: activate
preauditRequested: true
sap-client: 001
sap-language: EN
[BasicAuth]
{{username}}: {{password}}

<?xml version="1.0" encoding="UTF-8"?>
<adtcore:objectReferences xmlns:adtcore="http://www.sap.com/adt/core">
<adtcore:objectReference adtcore:uri="/sap/bc/adt/programs/programs/ztest2" adtcore:name="ZTEST2"/>
</adtcore:objectReferences>

HTTP 200
