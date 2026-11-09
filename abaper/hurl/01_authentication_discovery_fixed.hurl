# 🔐 Authentication & Discovery (Fixed for 401 issues)
# Complete collection of ABAP Development Tools (ADT) authentication and discovery endpoints
# Environment variables: SAP_HOST, SAP_USERNAME, SAP_PASSWORD, SAP_CLIENT

# Login Compatibility Check - Initial login and CSRF token fetch. Run this first!
# Using inline query parameters to avoid [QueryStringParams] issues
GET {{SAP_HOST}}/sap/bc/adt/compatibility/graph?sap-client={{SAP_CLIENT}}&sap-language=EN
Accept: application/xml
x-csrf-token: fetch
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
