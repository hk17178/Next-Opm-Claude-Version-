package biz

import (
	"fmt"
	"strings"
)

// ExprToken 表示条件表达式词法分析后的一个 token。
type ExprToken struct {
	Type  string // "field", "op", "value", "logic"
	Value string
}

// ExprCondition 表示一个解析后的条件子句（字段 操作符 值）。
type ExprCondition struct {
	Field    string
	Operator string // ==, !=, LIKE
	Value    string
}

// ParsedExpression 表示解析后的完整条件表达式。
type ParsedExpression struct {
	Conditions []ExprCondition
	Operators  []string // AND/OR，第 i 个 operator 连接第 i 和第 i+1 个条件
}

// ParseExpression 解析简单的条件表达式字符串，支持 AND/OR 连接符。
// 格式: field == "value" AND field != "value" OR field LIKE "pattern"
// 返回解析后的结构化表达式，供 SQL 查询构建使用。
func ParseExpression(expr string) (*ParsedExpression, error) {
	if expr == "" {
		return &ParsedExpression{}, nil
	}

	result := &ParsedExpression{}

	// 按 AND/OR 分割为多个条件子句
	parts := tokenizeByLogic(expr)

	for i, part := range parts {
		if i%2 == 1 {
			// 逻辑操作符
			op := strings.TrimSpace(part)
			if op != "AND" && op != "OR" {
				return nil, fmt.Errorf("invalid logical operator: %s", op)
			}
			result.Operators = append(result.Operators, op)
			continue
		}

		// 条件子句
		cond, err := parseCondition(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("invalid condition %q: %w", part, err)
		}
		result.Conditions = append(result.Conditions, *cond)
	}

	if len(result.Conditions) == 0 {
		return nil, fmt.Errorf("no conditions found in expression")
	}

	return result, nil
}

// tokenizeByLogic 按 AND/OR 关键词分割表达式，保留分隔符。
func tokenizeByLogic(expr string) []string {
	var parts []string
	current := ""

	words := strings.Fields(expr)
	for i := 0; i < len(words); i++ {
		upper := strings.ToUpper(words[i])
		if upper == "AND" || upper == "OR" {
			if current != "" {
				parts = append(parts, strings.TrimSpace(current))
				current = ""
			}
			parts = append(parts, upper)
		} else {
			if current != "" {
				current += " "
			}
			current += words[i]
		}
	}
	if current != "" {
		parts = append(parts, strings.TrimSpace(current))
	}

	return parts
}

// parseCondition 解析单个条件子句: field op "value"
func parseCondition(s string) (*ExprCondition, error) {
	// 支持的操作符
	operators := []string{"==", "!=", "LIKE"}

	for _, op := range operators {
		idx := strings.Index(s, op)
		if idx == -1 {
			continue
		}

		field := strings.TrimSpace(s[:idx])
		value := strings.TrimSpace(s[idx+len(op):])

		if field == "" {
			return nil, fmt.Errorf("empty field name")
		}

		// 去掉引号
		value = strings.Trim(value, "\"'")

		return &ExprCondition{
			Field:    field,
			Operator: op,
			Value:    value,
		}, nil
	}

	return nil, fmt.Errorf("no valid operator found (supports ==, !=, LIKE)")
}

// BuildSQLFromExpression 将解析后的表达式转换为 SQL WHERE 子句片段和参数列表。
// allowedFields 定义允许查询的字段白名单及其对应的数据库列名，防止 SQL 注入。
func BuildSQLFromExpression(expr *ParsedExpression, allowedFields map[string]string, startIdx int) (string, []any) {
	if expr == nil || len(expr.Conditions) == 0 {
		return "", nil
	}

	var clauses []string
	var args []any
	argIdx := startIdx

	for i, cond := range expr.Conditions {
		col, ok := allowedFields[cond.Field]
		if !ok {
			// 跳过不在白名单中的字段
			continue
		}

		var clause string
		switch cond.Operator {
		case "==":
			clause = fmt.Sprintf("%s = $%d", col, argIdx)
			args = append(args, cond.Value)
			argIdx++
		case "!=":
			clause = fmt.Sprintf("%s != $%d", col, argIdx)
			args = append(args, cond.Value)
			argIdx++
		case "LIKE":
			clause = fmt.Sprintf("%s ILIKE $%d", col, argIdx)
			args = append(args, "%"+cond.Value+"%")
			argIdx++
		default:
			continue
		}

		if i > 0 && i-1 < len(expr.Operators) {
			logicOp := expr.Operators[i-1]
			clauses = append(clauses, logicOp, clause)
		} else {
			clauses = append(clauses, clause)
		}
	}

	if len(clauses) == 0 {
		return "", nil
	}

	return "(" + strings.Join(clauses, " ") + ")", args
}
