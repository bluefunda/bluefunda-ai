# 📂 Repository Management
# Repository and object search functionality
# Environment variables: SAP_HOST, SAP_USERNAME, SAP_PASSWORD, SAP_CLIENT

# Get Object Types - Get available object types
GET {{SAP_HOST}}/sap/bc/adt/repository/informationsystem/objecttypes
Accept: application/xml
[QueryStringParams]
maxItemCount: 999
name: *
data: usedByProvider
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Search Objects - Search for objects with wildcard
GET {{SAP_HOST}}/sap/bc/adt/repository/informationsystem/search
Accept: application/*
[QueryStringParams]
operation: quickSearch
query: Z*
maxResults: 100
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Find Object Path - Find path for specific object
POST {{SAP_HOST}}/sap/bc/adt/repository/nodepath
Accept: application/xml
[QueryStringParams]
uri: /sap/bc/adt/programs/programs/z_test
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Node Contents - Get contents of a repository node
POST {{SAP_HOST}}/sap/bc/adt/repository/informationsystem/nodecontents
Accept: application/xml
[QueryStringParams]
parent_type: devc
parent_name: $TMP
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200
