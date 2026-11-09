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
