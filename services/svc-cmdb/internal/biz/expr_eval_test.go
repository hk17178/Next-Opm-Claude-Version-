package biz

import (
	"testing"
)

// --- ParseExpression Tests ---

func TestParseExpression_SingleCondition(t *testing.T) {
	expr, err := ParseExpression(`status == "active"`)
	if err != nil {
		t.Fatalf("ParseExpression error: %v", err)
	}
	if len(expr.Conditions) != 1 {
		t.Fatalf("conditions count = %d, want 1", len(expr.Conditions))
	}
	c := expr.Conditions[0]
	if c.Field != "status" {
		t.Errorf("field = %s, want status", c.Field)
	}
	if c.Operator != "==" {
		t.Errorf("operator = %s, want ==", c.Operator)
	}
	if c.Value != "active" {
		t.Errorf("value = %s, want active", c.Value)
	}
}

func TestParseExpression_ANDConditions(t *testing.T) {
	expr, err := ParseExpression(`asset_type == "server" AND status == "active"`)
	if err != nil {
		t.Fatalf("ParseExpression error: %v", err)
	}
	if len(expr.Conditions) != 2 {
		t.Fatalf("conditions count = %d, want 2", len(expr.Conditions))
	}
	if len(expr.Operators) != 1 {
		t.Fatalf("operators count = %d, want 1", len(expr.Operators))
	}
	if expr.Operators[0] != "AND" {
		t.Errorf("operator = %s, want AND", expr.Operators[0])
	}

	if expr.Conditions[0].Field != "asset_type" {
		t.Errorf("cond[0].field = %s, want asset_type", expr.Conditions[0].Field)
	}
	if expr.Conditions[1].Field != "status" {
		t.Errorf("cond[1].field = %s, want status", expr.Conditions[1].Field)
	}
}

func TestParseExpression_ORConditions(t *testing.T) {
	expr, err := ParseExpression(`grade == "S" OR grade == "A"`)
	if err != nil {
		t.Fatalf("ParseExpression error: %v", err)
	}
	if len(expr.Conditions) != 2 {
		t.Fatalf("conditions count = %d, want 2", len(expr.Conditions))
	}
	if expr.Operators[0] != "OR" {
		t.Errorf("operator = %s, want OR", expr.Operators[0])
	}
}

func TestParseExpression_MultipleANDConditions(t *testing.T) {
	expr, err := ParseExpression(`asset_type == "server" AND status == "active" AND grade == "S"`)
	if err != nil {
		t.Fatalf("ParseExpression error: %v", err)
	}
	if len(expr.Conditions) != 3 {
		t.Fatalf("conditions count = %d, want 3", len(expr.Conditions))
	}
	if len(expr.Operators) != 2 {
		t.Fatalf("operators count = %d, want 2", len(expr.Operators))
	}
}

func TestParseExpression_NotEqualOperator(t *testing.T) {
	expr, err := ParseExpression(`status != "retired"`)
	if err != nil {
		t.Fatalf("ParseExpression error: %v", err)
	}
	if expr.Conditions[0].Operator != "!=" {
		t.Errorf("operator = %s, want !=", expr.Conditions[0].Operator)
	}
	if expr.Conditions[0].Value != "retired" {
		t.Errorf("value = %s, want retired", expr.Conditions[0].Value)
	}
}

func TestParseExpression_LIKEOperator(t *testing.T) {
	expr, err := ParseExpression(`hostname LIKE "web"`)
	if err != nil {
		t.Fatalf("ParseExpression error: %v", err)
	}
	if expr.Conditions[0].Operator != "LIKE" {
		t.Errorf("operator = %s, want LIKE", expr.Conditions[0].Operator)
	}
	if expr.Conditions[0].Value != "web" {
		t.Errorf("value = %s, want web", expr.Conditions[0].Value)
	}
}

func TestParseExpression_EmptyString(t *testing.T) {
	expr, err := ParseExpression("")
	if err != nil {
		t.Fatalf("ParseExpression error: %v", err)
	}
	if len(expr.Conditions) != 0 {
		t.Errorf("conditions count = %d, want 0", len(expr.Conditions))
	}
}

func TestParseExpression_InvalidNoOperator(t *testing.T) {
	_, err := ParseExpression(`status active`)
	if err == nil {
		t.Fatal("expected error for missing operator")
	}
}

// --- BuildSQLFromExpression Tests ---

func TestBuildSQLFromExpression_SingleEquals(t *testing.T) {
	allowed := map[string]string{
		"status":     "status",
		"asset_type": "asset_type",
	}
	expr, _ := ParseExpression(`status == "active"`)

	sql, args := BuildSQLFromExpression(expr, allowed, 1)
	if sql != "(status = $1)" {
		t.Errorf("sql = %s, want (status = $1)", sql)
	}
	if len(args) != 1 || args[0] != "active" {
		t.Errorf("args = %v, want [active]", args)
	}
}

func TestBuildSQLFromExpression_ANDConditions(t *testing.T) {
	allowed := map[string]string{
		"status":     "status",
		"asset_type": "asset_type",
	}
	expr, _ := ParseExpression(`asset_type == "server" AND status == "active"`)

	sql, args := BuildSQLFromExpression(expr, allowed, 1)
	if sql != "(asset_type = $1 AND status = $2)" {
		t.Errorf("sql = %s, want (asset_type = $1 AND status = $2)", sql)
	}
	if len(args) != 2 {
		t.Errorf("args count = %d, want 2", len(args))
	}
}

func TestBuildSQLFromExpression_NotEqual(t *testing.T) {
	allowed := map[string]string{"status": "status"}
	expr, _ := ParseExpression(`status != "retired"`)

	sql, args := BuildSQLFromExpression(expr, allowed, 1)
	if sql != "(status != $1)" {
		t.Errorf("sql = %s, want (status != $1)", sql)
	}
	if len(args) != 1 || args[0] != "retired" {
		t.Errorf("args = %v, want [retired]", args)
	}
}

func TestBuildSQLFromExpression_LIKE(t *testing.T) {
	allowed := map[string]string{"hostname": "hostname"}
	expr, _ := ParseExpression(`hostname LIKE "web"`)

	sql, args := BuildSQLFromExpression(expr, allowed, 1)
	if sql != "(hostname ILIKE $1)" {
		t.Errorf("sql = %s, want (hostname ILIKE $1)", sql)
	}
	if len(args) != 1 || args[0] != "%web%" {
		t.Errorf("args = %v, want [%%web%%]", args)
	}
}

func TestBuildSQLFromExpression_UnknownFieldSkipped(t *testing.T) {
	allowed := map[string]string{"status": "status"}
	expr, _ := ParseExpression(`unknown_field == "value"`)

	sql, args := BuildSQLFromExpression(expr, allowed, 1)
	if sql != "" {
		t.Errorf("sql = %s, want empty (unknown field should be skipped)", sql)
	}
	if len(args) != 0 {
		t.Errorf("args count = %d, want 0", len(args))
	}
}

func TestBuildSQLFromExpression_CustomStartIdx(t *testing.T) {
	allowed := map[string]string{"status": "status"}
	expr, _ := ParseExpression(`status == "active"`)

	sql, args := BuildSQLFromExpression(expr, allowed, 5)
	if sql != "(status = $5)" {
		t.Errorf("sql = %s, want (status = $5)", sql)
	}
	if len(args) != 1 {
		t.Errorf("args count = %d, want 1", len(args))
	}
}

func TestBuildSQLFromExpression_Nil(t *testing.T) {
	sql, args := BuildSQLFromExpression(nil, nil, 1)
	if sql != "" {
		t.Errorf("sql = %s, want empty", sql)
	}
	if len(args) != 0 {
		t.Errorf("args count = %d, want 0", len(args))
	}
}
