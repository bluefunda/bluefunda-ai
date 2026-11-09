REPORT ZTEST2.

* Updated program source code via abaper CLI update command
* This demonstrates the update functionality

PARAMETERS: p_msg TYPE string DEFAULT 'Hello from updated program!'.

START-OF-SELECTION.
  WRITE: / 'Program ZTEST_METADATA_ONLY has been updated successfully',
         / 'New message:', p_msg,
         / 'Update date:', sy-datum,
         / 'Update time:', sy-uzeit,
         / 'Updated via abaper CLI update command'.
