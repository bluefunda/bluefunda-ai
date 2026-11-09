REPORT ZTEST_CLI_CREATE.

* Test program created via abaper CLI
* This demonstrates source code input from file

PARAMETERS: p_text TYPE string DEFAULT 'Hello from CLI created program!'.

START-OF-SELECTION.
  WRITE: / 'Program ZTEST_CLI_CREATE executed successfully'.
  WRITE: / 'Parameter value:', p_text.
  WRITE: / 'Current date:', sy-datum.
  WRITE: / 'Current time:', sy-uzeit.
  WRITE: / 'Created by abaper CLI with source file input'.