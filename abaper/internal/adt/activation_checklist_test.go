package adt

import "testing"

// These fixtures are the literal chkl:messages bodies SAP returned (HTTP 200
// in both cases) when live-testing activation of a Service Definition
// referencing an inactive CDS view — the bug that motivated this parsing:
// a 200 status alone does not mean the object activated.
const activationChecklistErrorBody = `<?xml version="1.0" encoding="utf-8"?><chkl:messages xmlns:chkl="http://www.sap.com/abapxml/checklist"><chkl:properties checkExecuted="true" activationExecuted="false" generationExecuted="false"/><msg objDescr="" type="W" line="0" href=""><shortText><txt>Activation was cancelled.</txt><txt>"Editing canceled" (EU 202)</txt></shortText></msg><msg objDescr="Service Definition ZODT_TESTSRV" type="E" line="1" href="/sap/bc/adt/ddic/srvd/sources/zodt_testsrv/source/main#start=3,9;end=3,22"><shortText><txt>Entity 'ZODT_TESTVIEW' does not exist</txt></shortText></msg></chkl:messages>`

const activationChecklistSuccessBody = `<?xml version="1.0" encoding="utf-8"?><chkl:messages xmlns:chkl="http://www.sap.com/abapxml/checklist"><chkl:properties checkExecuted="true" activationExecuted="true" generationExecuted="true"/></chkl:messages>`

// activationChecklistNotExecutedBody is the literal body SAP returned live
// when activation was attempted immediately after unlocking a just-created
// object: no messages at all, just checkExecuted="false". This is the real
// root cause behind CreateSRVD/CreateDDLS reporting "activated: true" while
// SAP actually left the object inactive — it must not be read as success.
const activationChecklistNotExecutedBody = `<?xml version="1.0" encoding="utf-8"?><chkl:messages xmlns:chkl="http://www.sap.com/abapxml/checklist"><chkl:properties checkExecuted="false" activationExecuted="false" generationExecuted="false"/></chkl:messages>`

func TestActivationChecklistOutcomeDistinguishesNotExecuted(t *testing.T) {
	checked, errText := activationChecklistOutcome([]byte(activationChecklistNotExecutedBody))
	if checked {
		t.Error("expected checkExecuted=false to be reported as not executed")
	}
	if errText != "" {
		t.Errorf("expected no error text when the check never ran, got: %q", errText)
	}
	// activationChecklistError alone (no outcome distinction) must not treat
	// this as a clean pass either, since it reports the same "no error text"
	// as a genuine success — this is exactly why callers use
	// activationChecklistOutcome/activate*WithRetry instead of trusting a nil
	// activationChecklistError as "activated".
	if err := activationChecklistError([]byte(activationChecklistNotExecutedBody)); err != nil {
		t.Errorf("activationChecklistError should not itself error on this body, got: %v", err)
	}
}

func TestActivationChecklistOutcomeDetectsSuccess(t *testing.T) {
	checked, errText := activationChecklistOutcome([]byte(activationChecklistSuccessBody))
	if !checked {
		t.Error("expected checkExecuted=true to be reported as executed")
	}
	if errText != "" {
		t.Errorf("expected no error text for a genuine success, got: %q", errText)
	}
}

func TestActivationChecklistErrorDetectsFailure(t *testing.T) {
	err := activationChecklistError([]byte(activationChecklistErrorBody))
	if err == nil {
		t.Fatal("expected an error for a checklist body containing a type=\"E\" message, got nil")
	}
	if got := err.Error(); got != "Entity 'ZODT_TESTVIEW' does not exist" {
		t.Errorf("expected the error-severity message text only (warnings excluded), got: %q", got)
	}
}

func TestActivationChecklistErrorAllowsSuccess(t *testing.T) {
	if err := activationChecklistError([]byte(activationChecklistSuccessBody)); err != nil {
		t.Errorf("expected no error for a checklist body with no error messages, got: %v", err)
	}
}

func TestIsActivationError(t *testing.T) {
	cases := map[string]bool{
		"E": true, "A": true, "error": true,
		"W": false, "S": false, "": false,
	}
	for severity, want := range cases {
		if got := isActivationError(severity); got != want {
			t.Errorf("isActivationError(%q) = %v, want %v", severity, got, want)
		}
	}
}
