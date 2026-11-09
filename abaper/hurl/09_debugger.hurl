# 🐞 Debugger
# Debugging functionality - listeners, breakpoints, stack trace
# Environment variables: SAP_HOST, SAP_USERNAME, SAP_PASSWORD, SAP_CLIENT

# Check Debug Listeners - Check active debug listeners
GET {{SAP_HOST}}/sap/bc/adt/debugger/listeners
[QueryStringParams]
debuggingMode: user
requestUser: {{SAP_USERNAME}}
terminalId: 12345
ideId: 67890
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Set Breakpoint - Set line breakpoint in code
POST {{SAP_HOST}}/sap/bc/adt/debugger/breakpoints
Content-Type: application/xml
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
<?xml version="1.0"?>
<dbg:breakpoints xmlns:dbg="http://www.sap.com/adt/debugger" scope="external" debuggingMode="user" requestUser="{{SAP_USERNAME}}" terminalId="12345" ideId="67890">
<syncScope mode="full"/>
<breakpoint kind="line" clientId="test" adtcore:uri="/sap/bc/adt/programs/programs/z_test/source/main#start=5" xmlns:adtcore="http://www.sap.com/adt/core"/>
</dbg:breakpoints>
```

HTTP 200

###

# Get Debug Stack - Get current debug stack trace
GET {{SAP_HOST}}/sap/bc/adt/debugger/stack
[QueryStringParams]
method: getStack
semanticURIs: true
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200
