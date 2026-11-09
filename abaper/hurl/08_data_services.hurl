# 📊 Data & Services
# Data preview and SQL query execution
# Environment variables: SAP_HOST, SAP_USERNAME, SAP_PASSWORD, SAP_CLIENT

# Table Contents - Preview table data
POST {{SAP_HOST}}/sap/bc/adt/datapreview/ddic
Content-Type: text/plain
x-csrf-token: {{csrf_token}}
[QueryStringParams]
rowNumber: 100
ddicEntityName: MARA
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Run SQL Query - Execute freestyle SQL query
POST {{SAP_HOST}}/sap/bc/adt/datapreview/freestyle
Content-Type: text/plain
x-csrf-token: {{csrf_token}}
[QueryStringParams]
rowNumber: 100
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
SELECT * FROM MARA WHERE MATNR LIKE 'T%'
```

HTTP 200
