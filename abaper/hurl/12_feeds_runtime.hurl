# 📋 Feeds & Runtime
# Feeds and runtime information
# Environment variables: SAP_HOST, SAP_USERNAME, SAP_PASSWORD, SAP_CLIENT

# Get Feeds - Get available feeds
GET {{SAP_HOST}}/sap/bc/adt/feeds
Accept: application/atom+xml;type=feed
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Get Dumps Feed - Get runtime dumps feed
GET {{SAP_HOST}}/sap/bc/adt/runtime/dumps
Accept: application/atom+xml;type=feed
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200
