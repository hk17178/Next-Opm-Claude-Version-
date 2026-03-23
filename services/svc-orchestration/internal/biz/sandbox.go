package biz

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"text/template"
	"time"
)

// 默认脚本执行超时时间
const defaultScriptTimeout = 30 * time.Second

// dangerousPatterns 定义禁止执行的危险命令模式。
// 包括破坏性文件操作、磁盘格式化、fork 炸弹等。
var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`rm\s+-[^\s]*r[^\s]*f[^\s]*\s+/\s*$`), // rm -rf /
	regexp.MustCompile(`rm\s+-[^\s]*f[^\s]*r[^\s]*\s+/\s*$`), // rm -fr /
	regexp.MustCompile(`mkfs\b`),                               // 磁盘格式化
	regexp.MustCompile(`dd\s+.*of=/dev/`),                      // 直接写设备
	regexp.MustCompile(`format\s+[A-Za-z]:`),                   // Windows 格式化
	regexp.MustCompile(`:\(\)\s*\{\s*:\|:\s*&\s*\}\s*;:`),      // fork 炸弹
	regexp.MustCompile(`>\s*/dev/sda`),                         // 覆写磁盘
	regexp.MustCompile(`chmod\s+-R\s+777\s+/\s*$`),            // 递归修改根目录权限
}

// ScriptSandbox 提供安全的脚本执行沙箱环境。
// 支持危险命令检查、执行超时控制和变量注入。
type ScriptSandbox struct {
	Timeout time.Duration // 脚本执行超时时间
}

// NewScriptSandbox 创建脚本沙箱实例，使用默认超时时间。
func NewScriptSandbox() *ScriptSandbox {
	return &ScriptSandbox{
		Timeout: defaultScriptTimeout,
	}
}

// ScriptResult 包含脚本执行的结果信息。
type ScriptResult struct {
	Stdout   string `json:"stdout"`    // 标准输出
	Stderr   string `json:"stderr"`    // 标准错误输出
	ExitCode int    `json:"exit_code"` // 退出码
}

// Execute 在沙箱中执行 Shell 脚本。
// 执行流程：危险命令检查 → 变量替换 → 超时控制执行 → 捕获输出。
//
// 参数：
//   - script: 要执行的 Shell 脚本内容
//   - variables: 用于模板替换的变量映射，支持 {{.VarName}} 语法
//
// 返回脚本执行结果和可能的错误（危险命令或超时等）。
func (s *ScriptSandbox) Execute(script string, variables map[string]string) (*ScriptResult, error) {
	// 第一步：检查是否包含危险命令
	if err := s.checkDangerousCommands(script); err != nil {
		return nil, err
	}

	// 第二步：进行变量替换（支持 {{.VarName}} 模板语法）
	rendered, err := s.renderTemplate(script, variables)
	if err != nil {
		return nil, fmt.Errorf("模板渲染失败: %w", err)
	}

	// 第三步：在超时上下文中执行脚本
	ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", rendered)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	result := &ScriptResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		// 检查是否因超时导致
		if ctx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("脚本执行超时（%v）", s.Timeout)
		}
		// 获取退出码
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		return result, fmt.Errorf("脚本执行失败: %w", err)
	}

	result.ExitCode = 0
	return result, nil
}

// checkDangerousCommands 检查脚本内容是否包含危险命令模式。
// 若匹配任一危险模式，则拒绝执行并返回错误。
func (s *ScriptSandbox) checkDangerousCommands(script string) error {
	// 先对脚本进行规范化处理（去除多余空格等）
	normalized := strings.TrimSpace(script)

	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(normalized) {
			return fmt.Errorf("检测到危险命令，拒绝执行: %s", pattern.String())
		}
	}
	return nil
}

// renderTemplate 使用 Go 模板引擎进行变量替换。
// 支持 {{.VarName}} 语法，将变量映射中的值替换到脚本模板中。
func (s *ScriptSandbox) renderTemplate(script string, variables map[string]string) (string, error) {
	if len(variables) == 0 {
		return script, nil
	}

	tmpl, err := template.New("script").Parse(script)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, variables); err != nil {
		return "", err
	}

	return buf.String(), nil
}
