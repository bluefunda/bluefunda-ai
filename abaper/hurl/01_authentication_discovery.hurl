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

###

# ADT Discovery - Discover available ADT services
GET {{host}}/sap/bc/adt/discovery
Accept: application/atomsvc+xml
[BasicAuth]
{{username}}: {{password}}

HTTP 200

###

# ADT Core Discovery - Discover ADT core services
GET {{host}}/sap/bc/adt/core/discovery
Accept: application/atomsvc+xml
[BasicAuth]
{{username}}: {{password}}

HTTP 200

###

# Get Reentrance Ticket - Get reentrance ticket for debugging
GET {{host}}/sap/bc/adt/security/reentranceticket
Accept: text/plain
[BasicAuth]
{{username}}: {{password}}

HTTP 200

###

# Logout - Logout and clear session
GET {{host}}/sap/public/bc/icf/logoff
[BasicAuth]
{{username}}: {{password}}

HTTP 200
