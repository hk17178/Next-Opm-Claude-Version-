package biz

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSandbox_NormalExecution 测试正常脚本执行。
func TestSandbox_NormalExecution(t *testing.T) {
	sandbox := NewScriptSandbox()

	result, err := sandbox.Execute("echo hello world", nil)
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "hello world")
	assert.Equal(t, 0, result.ExitCode)
}

// TestSandbox_DangerousCommand_RmRf 测试 rm -rf / 应被拦截。
func TestSandbox_DangerousCommand_RmRf(t *testing.T) {
	sandbox := NewScriptSandbox()

	_, err := sandbox.Execute("rm -rf /", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "危险命令")
}

// TestSandbox_DangerousCommand_Mkfs 测试 mkfs 应被拦截。
func TestSandbox_DangerousCommand_Mkfs(t *testing.T) {
	sandbox := NewScriptSandbox()

	_, err := sandbox.Execute("mkfs.ext4 /dev/sda1", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "危险命令")
}

// TestSandbox_DangerousCommand_DdDev 测试 dd of=/dev/ 应被拦截。
func TestSandbox_DangerousCommand_DdDev(t *testing.T) {
	sandbox := NewScriptSandbox()

	_, err := sandbox.Execute("dd if=/dev/zero of=/dev/sda bs=1M", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "危险命令")
}

// TestSandbox_VariableSubstitution 测试变量替换功能。
func TestSandbox_VariableSubstitution(t *testing.T) {
	sandbox := NewScriptSandbox()

	variables := map[string]string{
		"Name": "OpsNexus",
	}

	result, err := sandbox.Execute("echo Hello {{.Name}}", variables)
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "Hello OpsNexus")
}

// TestSandbox_Timeout 测试脚本超时控制。
func TestSandbox_Timeout(t *testing.T) {
	sandbox := NewScriptSandbox()
	sandbox.Timeout = 1 * time.Second

	_, err := sandbox.Execute("sleep 10", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "超时")
}

// TestSandbox_NonZeroExit 测试脚本返回非零退出码。
func TestSandbox_NonZeroExit(t *testing.T) {
	sandbox := NewScriptSandbox()

	result, err := sandbox.Execute("exit 42", nil)
	assert.Error(t, err)
	assert.Equal(t, 42, result.ExitCode)
}

// TestSandbox_EmptyVariables 测试空变量时应正常执行。
func TestSandbox_EmptyVariables(t *testing.T) {
	sandbox := NewScriptSandbox()

	result, err := sandbox.Execute("echo no vars", map[string]string{})
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "no vars")
}
