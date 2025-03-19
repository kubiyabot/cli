package kubiya

import (
	"testing"
)

func TestValidateVariableType(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		expectedType string
		wantValid    bool
	}{
		// String tests
		{"valid_string", "hello", "string", true},
		{"number_as_string", "123", "string", true},
		{"json_as_string", `{"key":"value"}`, "string", true},

		// Number tests
		{"valid_integer", "42", "number", true},
		{"valid_float", "3.14", "number", true},
		{"negative_number", "-10", "number", true},
		{"invalid_number", "hello", "number", false},

		// Boolean tests
		{"valid_bool_true", "true", "boolean", true},
		{"valid_bool_false", "false", "boolean", true},
		{"valid_bool_yes", "yes", "boolean", true},
		{"valid_bool_no", "no", "boolean", true},
		{"valid_bool_1", "1", "boolean", true},
		{"valid_bool_0", "0", "boolean", true},
		{"invalid_bool", "hello", "boolean", false},
		{"invalid_bool_number", "42", "boolean", false},

		// Array/list tests
		{"valid_array", `["one", "two", "three"]`, "array", true},
		{"valid_empty_array", `[]`, "array", true},
		{"invalid_array", "hello", "array", false},
		{"invalid_array_json", `{"key":"value"}`, "array", false},

		// Object/map tests
		{"valid_object", `{"name":"John", "age":30}`, "object", true},
		{"valid_empty_object", `{}`, "object", true},
		{"invalid_object", "hello", "object", false},
		{"invalid_object_json", `["one", "two"]`, "object", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, errMsg := validateVariableType(tt.value, tt.expectedType)
			if valid != tt.wantValid {
				t.Errorf("validateVariableType() = %v, want %v, error: %s", valid, tt.wantValid, errMsg)
			}
		})
	}
}
