# 🧪 Testing
# Unit testing and ABAP class execution
# Environment variables: SAP_HOST, SAP_USERNAME, SAP_PASSWORD, SAP_CLIENT

# Run Unit Tests - Execute unit tests for specified objects
POST {{SAP_HOST}}/sap/bc/adt/abapunit/testruns
Content-Type: application/vnd.sap.adt.abapunit.testruns.v4+xml
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
<?xml version="1.0"?>
<aunit:runConfiguration xmlns:aunit="http://www.sap.com/adt/aunit">
<aunit:external><aunit:coverage active="true"/></aunit:external>
<aunit:options><aunit:uriType value="semantic"/></aunit:options>
<aunit:objectSets><aunit:objectSet kind="inclusive">
<adtcore:objectReferences xmlns:adtcore="http://www.sap.com/adt/core">
<adtcore:objectReference adtcore:uri="/sap/bc/adt/oo/classes/zcl_test"/>
</adtcore:objectReferences></aunit:objectSet></aunit:objectSets>
</aunit:runConfiguration>
```

HTTP 200

###

# Run ABAP Class - Execute ABAP class (if_oo_adt_classrun)
POST {{SAP_HOST}}/sap/bc/adt/oo/classrun/ZCL_TEST_CLASS
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200
