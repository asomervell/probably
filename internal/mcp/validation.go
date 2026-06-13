package mcp

import (
	"encoding/json"
	"fmt"
)

// ValidationHandler validates tool inputs
type ValidationHandler struct{}

// NewValidationHandler creates a new validation handler
func NewValidationHandler() *ValidationHandler {
	return &ValidationHandler{}
}

// ValidateToolArguments validates tool arguments against the tool's input schema
func (v *ValidationHandler) ValidateToolArguments(arguments json.RawMessage, tool *ToolDefinition) error {
	// Parse arguments
	var args map[string]interface{}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Get schema
	schema, ok := tool.InputSchema["properties"].(map[string]interface{})
	if !ok {
		// No properties to validate
		return nil
	}

	// Validate required fields
	if required, ok := tool.InputSchema["required"].([]interface{}); ok {
		for _, req := range required {
			if reqStr, ok := req.(string); ok {
				if _, exists := args[reqStr]; !exists {
					return fmt.Errorf("missing required field: %s", reqStr)
				}
			}
		}
	}

	// Validate each argument
	for key, value := range args {
		prop, exists := schema[key]
		if !exists {
			// Unknown field - allow it for flexibility
			continue
		}

		propMap, ok := prop.(map[string]interface{})
		if !ok {
			continue
		}

		// Type validation
		if expectedType, ok := propMap["type"].(string); ok {
			if err := v.validateType(key, value, expectedType); err != nil {
				return err
			}
		}

		// Enum validation
		if enum, ok := propMap["enum"].([]interface{}); ok {
			if err := v.validateEnum(key, value, enum); err != nil {
				return err
			}
		}
	}

	return nil
}

func (v *ValidationHandler) validateType(fieldName string, value interface{}, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field %s must be a string", fieldName)
		}
	case "integer":
		// JSON numbers can be float64, so check if it's a whole number
		switch val := value.(type) {
		case float64:
			if val != float64(int64(val)) {
				return fmt.Errorf("field %s must be an integer", fieldName)
			}
		case int, int64:
			// OK
		default:
			return fmt.Errorf("field %s must be an integer", fieldName)
		}
	case "number":
		switch value.(type) {
		case float64, int, int64:
			// OK
		default:
			return fmt.Errorf("field %s must be a number", fieldName)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("field %s must be a boolean", fieldName)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("field %s must be an object", fieldName)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("field %s must be an array", fieldName)
		}
	}
	return nil
}

func (v *ValidationHandler) validateEnum(fieldName string, value interface{}, enum []interface{}) error {
	for _, enumVal := range enum {
		if value == enumVal {
			return nil
		}
	}
	return fmt.Errorf("field %s must be one of: %v", fieldName, enum)
}
