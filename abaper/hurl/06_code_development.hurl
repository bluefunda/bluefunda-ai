# 💻 Code Development
# Code development tools - syntax check, code completion, pretty print
# Environment variables: SAP_HOST, SAP_USERNAME, SAP_PASSWORD, SAP_CLIENT

# Syntax Check - Check ABAP code syntax
POST {{SAP_HOST}}/sap/bc/adt/checkruns
Content-Type: text/plain
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
REPORT z_test.
WRITE: 'Hello World'.
```

HTTP 200

###

# Code Completion - Get code completion suggestions
POST {{SAP_HOST}}/sap/bc/adt/abap/completion/semantics
Content-Type: text/plain
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
REPORT z_test.
WR
```

HTTP 200

###

# Pretty Print - Format ABAP code
POST {{SAP_HOST}}/sap/bc/adt/abap/formatter
Content-Type: text/plain
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
report z_test.
write:'hello'.
```

HTTP 200
