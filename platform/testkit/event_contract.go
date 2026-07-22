package testkit

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
)

// AssertEventContract validates the portable JSON Schema subset every event
// producer test can prove without a language-specific schema engine: required
// envelope/data fields, const values, declared data properties and JSON types.
// It returns the decoded envelope for service-specific assertions.
func AssertEventContract(tb testing.TB, eventType string, payload []byte) map[string]any {
	tb.Helper()
	schema := readEventContract(tb, eventType)
	var event map[string]any
	if err := json.Unmarshal(payload, &event); err != nil {
		tb.Fatalf("decode published %s event: %v", eventType, err)
	}
	assertSchemaValue(tb, eventType, schema, event)
	return event
}

// assertSchemaValue recursively enforces the JSON Schema vocabulary used by
// AuraEDU event contracts. Keeping this traversal in the shared testkit avoids
// service-local validators silently accepting constraints they do not inspect.
func assertSchemaValue(tb testing.TB, label string, schema map[string]any, value any) {
	tb.Helper()
	assertConst(tb, label, schema, value)
	assertEnum(tb, label, schema, value)
	assertJSONType(tb, label, schema, value)
	assertStringRules(tb, label, schema, value)
	assertNumberRules(tb, label, schema, value)

	switch typed := value.(type) {
	case map[string]any:
		assertRequired(tb, label, schema, typed)
		assertAnyOfRequired(tb, label, schema, typed)
		properties := map[string]any{}
		if rawProperties, exists := schema["properties"]; exists {
			properties = requireObject(tb, rawProperties, label+" properties")
		}
		if additional, declared := schema["additionalProperties"].(bool); declared && !additional {
			for name := range typed {
				if _, ok := properties[name]; !ok {
					tb.Errorf("%s contains undeclared field %q", label, name)
				}
			}
		}
		for name, child := range typed {
			rawProperty, declared := properties[name]
			if !declared {
				continue
			}
			property := requireObject(tb, rawProperty, label+" property "+name)
			assertSchemaValue(tb, label+"."+name, property, child)
		}
	case []any:
		assertArrayRules(tb, label, schema, typed)
		itemSchema, declared := schema["items"].(map[string]any)
		if !declared {
			return
		}
		for index, item := range typed {
			assertSchemaValue(tb, fmt.Sprintf("%s[%d]", label, index), itemSchema, item)
		}
	}
}

func assertAnyOfRequired(tb testing.TB, label string, schema, value map[string]any) {
	tb.Helper()
	rawBranches, constrained := schema["anyOf"]
	if !constrained {
		return
	}
	branches, ok := rawBranches.([]any)
	if !ok {
		tb.Fatalf("%s anyOf is %T, want array", label, rawBranches)
	}
	for _, rawBranch := range branches {
		branch := requireObject(tb, rawBranch, label+" anyOf branch")
		matched := true
		for _, name := range requireStrings(tb, branch["required"], label+" anyOf required") {
			if _, present := value[name]; !present {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	tb.Errorf("%s does not satisfy any anyOf required-field alternative", label)
}

func assertArrayRules(tb testing.TB, label string, schema map[string]any, value []any) {
	tb.Helper()
	if rawMinimum, constrained := schema["minItems"]; constrained {
		minimum, valid := rawMinimum.(float64)
		if !valid {
			tb.Fatalf("%s minItems is %T, want number", label, rawMinimum)
		}
		if len(value) < int(minimum) {
			tb.Errorf("%s has %d items, want at least %d", label, len(value), int(minimum))
		}
	}
	if rawMaximum, constrained := schema["maxItems"]; constrained {
		maximum, valid := rawMaximum.(float64)
		if !valid {
			tb.Fatalf("%s maxItems is %T, want number", label, rawMaximum)
		}
		if len(value) > int(maximum) {
			tb.Errorf("%s has %d items, want at most %d", label, len(value), int(maximum))
		}
	}
	unique, uniqueDeclared := schema["uniqueItems"].(bool)
	if _, exists := schema["uniqueItems"]; exists && !uniqueDeclared {
		tb.Fatalf("%s uniqueItems is %T, want boolean", label, schema["uniqueItems"])
	}
	if unique {
		for left := range value {
			for right := left + 1; right < len(value); right++ {
				if reflect.DeepEqual(value[left], value[right]) {
					tb.Errorf("%s contains duplicate items at indexes %d and %d", label, left, right)
				}
			}
		}
	}
}

func assertNumberRules(tb testing.TB, label string, schema map[string]any, value any) {
	tb.Helper()
	number, ok := value.(float64)
	if !ok {
		return
	}
	if rawMinimum, constrained := schema["minimum"]; constrained {
		minimum, valid := rawMinimum.(float64)
		if !valid {
			tb.Fatalf("%s minimum is %T, want number", label, rawMinimum)
		}
		if number < minimum {
			tb.Errorf("%s is %v, want at least %v", label, number, minimum)
		}
	}
	if rawMaximum, constrained := schema["maximum"]; constrained {
		maximum, valid := rawMaximum.(float64)
		if !valid {
			tb.Fatalf("%s maximum is %T, want number", label, rawMaximum)
		}
		if number > maximum {
			tb.Errorf("%s is %v, want at most %v", label, number, maximum)
		}
	}
	if rawMinimum, constrained := schema["exclusiveMinimum"]; constrained {
		minimum, valid := rawMinimum.(float64)
		if !valid {
			tb.Fatalf("%s exclusiveMinimum is %T, want number", label, rawMinimum)
		}
		if number <= minimum {
			tb.Errorf("%s is %v, want greater than %v", label, number, minimum)
		}
	}
}

func readEventContract(tb testing.TB, eventType string) map[string]any {
	tb.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		tb.Fatal("resolve event-contract test helper source path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "..", ".."))
	contractPath := filepath.Join(repoRoot, "contracts", "events", eventType+".json")
	raw, err := os.ReadFile(contractPath)
	if err != nil {
		tb.Fatalf("read %s contract: %v", eventType, err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		tb.Fatalf("decode %s contract: %v", eventType, err)
	}
	return schema
}

func assertRequired(tb testing.TB, label string, schema, value map[string]any) {
	tb.Helper()
	for _, name := range requireStrings(tb, schema["required"], label+" required") {
		if _, ok := value[name]; !ok {
			tb.Errorf("%s is missing required field %q", label, name)
		}
	}
}

func assertConst(tb testing.TB, label string, schema map[string]any, value any) {
	tb.Helper()
	constant, constrained := schema["const"]
	if constrained && constant != value {
		tb.Errorf("%s is %v, want const %v", label, value, constant)
	}
}

func assertEnum(tb testing.TB, label string, schema map[string]any, value any) {
	tb.Helper()
	rawValues, constrained := schema["enum"]
	if !constrained {
		return
	}
	values, ok := rawValues.([]any)
	if !ok {
		tb.Fatalf("%s enum is %T, want array", label, rawValues)
	}
	for _, candidate := range values {
		if reflect.DeepEqual(candidate, value) {
			return
		}
	}
	tb.Errorf("%s is %v, want one of %v", label, value, values)
}

func assertJSONType(tb testing.TB, label string, schema map[string]any, value any) {
	tb.Helper()
	rawType, constrained := schema["type"]
	if !constrained {
		return
	}
	accepted := make([]string, 0, 2)
	switch typed := rawType.(type) {
	case string:
		accepted = append(accepted, typed)
	case []any:
		for _, item := range typed {
			if name, ok := item.(string); ok {
				accepted = append(accepted, name)
			}
		}
	}
	for _, name := range accepted {
		if matchesJSONType(name, value) {
			return
		}
	}
	tb.Errorf("%s has JSON value %v (%T), want type %v", label, value, value, rawType)
}

func matchesJSONType(name string, value any) bool {
	switch name {
	case "null":
		return value == nil
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := value.(float64)
		return ok
	case "integer":
		number, ok := value.(float64)
		return ok && math.Trunc(number) == number
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	default:
		return false
	}
}

func assertStringRules(tb testing.TB, label string, schema map[string]any, value any) {
	tb.Helper()
	text, ok := value.(string)
	if !ok {
		return
	}
	if rawMinimum, constrained := schema["minLength"]; constrained {
		minimum, valid := rawMinimum.(float64)
		if !valid {
			tb.Fatalf("%s minLength is %T, want number", label, rawMinimum)
		}
		if len([]rune(text)) < int(minimum) {
			tb.Errorf("%s has length %d, want at least %d", label, len([]rune(text)), int(minimum))
		}
	}
	format, formatDeclared := schema["format"].(string)
	if _, exists := schema["format"]; exists && !formatDeclared {
		tb.Fatalf("%s format is %T, want string", label, schema["format"])
	}
	switch format {
	case "uuid":
		if _, err := uuid.Parse(text); err != nil {
			tb.Errorf("%s is not a UUID: %v", label, err)
		}
	case "date-time":
		if _, err := time.Parse(time.RFC3339, text); err != nil {
			tb.Errorf("%s is not RFC3339 date-time: %v", label, err)
		}
	case "date":
		if _, err := time.Parse(time.DateOnly, text); err != nil {
			tb.Errorf("%s is not ISO date: %v", label, err)
		}
	}
}

func requireObject(tb testing.TB, value any, label string) map[string]any {
	tb.Helper()
	result, ok := value.(map[string]any)
	if !ok {
		tb.Fatalf("%s is %T, want object", label, value)
	}
	return result
}

func requireStrings(tb testing.TB, value any, label string) []string {
	tb.Helper()
	if value == nil {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		tb.Fatalf("%s is %T, want array", label, value)
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			tb.Fatalf("%s contains %T, want string", label, item)
		}
		result = append(result, text)
	}
	return result
}
