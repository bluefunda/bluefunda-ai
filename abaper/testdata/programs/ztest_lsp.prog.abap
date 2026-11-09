REPORT ztest_lsp.

DATA lv_name TYPE string.
DATA lv_count TYPE i.

IF lv_name IS INITIAL.
  WRITE lv_name.
  lv_count = lv_count + 1.

LOOP AT lt_data INTO ls_data.
  MOVE ls_data TO lv_name.
ENDLOOP.

CASE lv_count.
  WHEN 1.
    WRITE 'One'.
  WHEN 2.
    WRITE 'Two'.
ENDCASE.
