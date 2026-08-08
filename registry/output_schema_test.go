package registry

import "testing"

// TestOutputSchemaType_Unset and TestOutputSchemaProperties_Unset cover the
// "no schema at all" case directly; TestTypedHandler_OutputSchema
// (handler package) covers the populated case end-to-end through a real
// TypedHandler tool.
func TestOutputSchemaType_Unset(t *testing.T) {
	tool := Tool{Name: "no_schema"}
	if _, ok := OutputSchemaType(tool); ok {
		t.Error("OutputSchemaType: expected undeclared on a Tool with no OutputSchema")
	}
}

func TestOutputSchemaProperties_Unset(t *testing.T) {
	tool := Tool{Name: "no_schema"}
	if _, ok := OutputSchemaProperties(tool); ok {
		t.Error("OutputSchemaProperties: expected undeclared on a Tool with no OutputSchema")
	}
}
