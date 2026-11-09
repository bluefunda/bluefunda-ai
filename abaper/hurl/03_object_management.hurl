# 📄 Object Management
# Object creation, modification, locking and source management
# Environment variables: SAP_HOST, SAP_USERNAME, SAP_PASSWORD, SAP_CLIENT

# Get Object Structure - Get object structure metadata
GET {{SAP_HOST}}/sap/bc/adt/programs/programs/z_test_program
Accept: application/*
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Get Object Source - Get source code of an object
GET {{SAP_HOST}}/sap/bc/adt/programs/programs/z_test_program/source/main
Accept: text/plain
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Lock Object - Lock object for modification
POST {{SAP_HOST}}/sap/bc/adt/programs/programs/ztest1
Accept: application/*
X-CSRF-Token: {{csrf_token}}
[QueryStringParams]
_action: LOCK
accessMode: MODIFY
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200
[Captures]
lock_handle: xpath "//LOCK_HANDLE/text()"

###

# Update Object Source - Update source code of an object
PUT {{SAP_HOST}}/sap/bc/adt/programs/programs/z_test_program/source/main
Content-Type: text/plain; charset=utf-8
[QueryStringParams]
lockHandle: {{lock_handle}}
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
REPORT z_test_program.

WRITE: 'Hello from ADT API!'.
```

HTTP 200

###

# Unlock Object - Unlock object after modification
POST {{SAP_HOST}}/sap/bc/adt/programs/programs/z_test_program
[QueryStringParams]
_action: UNLOCK
lockHandle: {{lock_handle}}
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Create Program - Create a new ABAP program
POST {{SAP_HOST}}/sap/bc/adt/programs/programs
Content-Type: application/*
x-csrf-token: {{csrf_token}}
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
<?xml version="1.0" encoding="UTF-8"?>
<program:abapProgram xmlns:program="http://www.sap.com/adt/programs/programs"
  xmlns:adtcore="http://www.sap.com/adt/core"
  adtcore:description="My Test Program"
  adtcore:name="Z_MY_TEST_PROGRAM" 
  adtcore:type="PROG/P"
  adtcore:responsible="DEVELOPER">
  <adtcore:packageRef adtcore:name="ZTEST"/>
</program:abapProgram>
```

HTTP 201

###

# Create Class - Create a new ABAP class
POST {{SAP_HOST}}/sap/bc/adt/oo/classes
Content-Type: application/*
x-csrf-token: {{csrf_token}}
[QueryStringParams]
corrNr: A4HK900120
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
<?xml version="1.0" encoding="UTF-8"?>
<class:abapClass xmlns:class="http://www.sap.com/adt/oo/classes"
  xmlns:adtcore="http://www.sap.com/adt/core"
  adtcore:description="My Test Class"
  adtcore:name="ZCL_MY_TEST_CLASS" 
  adtcore:type="CLAS/OC"
  adtcore:responsible="PHANIKUMARPS">
  <adtcore:packageRef adtcore:name="ZBDA"/>
</class:abapClass>
```

HTTP 201

###

# Update Program - Update program source with transport
PUT {{SAP_HOST}}/sap/bc/adt/programs/programs/ztest1/source/main
Content-Type: text/plain; charset=utf-8
x-csrf-token: {{csrf_token}}
[QueryStringParams]
lockHandle: 480AB624CFAD5AF60525DC3D79789AEFAC9B40E8
corrNr: A4HK900122
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

```
REPORT ztest1.

WRITE: 'Hello World!'.

START-OF-SELECTION.
  WRITE: / 'This is my test program'.
  WRITE: / 'Created via ADT API'.
```

HTTP 200

###

# Validation - Validate object names
POST {{SAP_HOST}}/sap/bc/adt/oo/validation/objectname
Accept: application/*
X-CSRF-Token: {{csrf_token}}
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200

###

# Lock Object (without script) - Alternative lock method
POST {{SAP_HOST}}/sap/bc/adt/programs/programs/ztest1
Accept: application/*
X-CSRF-Token: {{csrf_token}}
[QueryStringParams]
_action: LOCK
accessMode: MODIFY
[BasicAuth]
{{SAP_USERNAME}}: {{SAP_PASSWORD}}

HTTP 200
