/**
 * 工作流编辑页面 - 支持新建和编辑模式
 *
 * 功能：
 * - 基本信息区：名称、描述、触发类型选择（手动/告警触发/定时）
 * - 定时触发时显示 Cron 表达式输入框
 * - 步骤列表编辑器：添加/删除/上移/下移步骤，每种类型有不同的配置表单
 * - 变量区：表格式定义工作流变量
 * - 保存/取消按钮
 */
import React, { useState, useEffect, useCallback } from 'react';
import {
  Card, Form, Input, Select, Button, Space, Collapse, Typography, InputNumber,
  Table, Modal, message, Tooltip, Divider, Row, Col, Tag,
} from 'antd';
import {
  PlusOutlined, DeleteOutlined, ArrowUpOutlined, ArrowDownOutlined,
  SaveOutlined, RollbackOutlined, CodeOutlined, AuditOutlined,
  BranchesOutlined, ClockCircleOutlined, BellOutlined,
} from '@ant-design/icons';
import { useNavigate, useParams } from 'react-router-dom';
import {
  getWorkflow, createWorkflow, updateWorkflow,
  type WorkflowStep, type WorkflowVariable, type StepType, type TriggerType,
  type NotifyChannel, type ApprovalTimeoutAction,
} from '../api/orchestration';

const { Text, Title } = Typography;
const { TextArea } = Input;
const { Panel } = Collapse;

/** 步骤类型选项配置 */
const STEP_TYPE_OPTIONS: Array<{
  value: StepType;
  label: string;
  icon: React.ReactNode;
  description: string;
}> = [
  { value: 'script', label: '脚本执行', icon: <CodeOutlined />, description: '执行 Shell 脚本或命令' },
  { value: 'approval', label: '人工审批', icon: <AuditOutlined />, description: '需要人工确认后继续' },
  { value: 'condition', label: '条件判断', icon: <BranchesOutlined />, description: '根据条件表达式分支执行' },
  { value: 'wait', label: '等待', icon: <ClockCircleOutlined />, description: '等待指定时间后继续' },
  { value: 'notify', label: '通知', icon: <BellOutlined />, description: '发送通知到指定渠道' },
];

/** 步骤类型图标映射 */
const STEP_TYPE_ICONS: Record<StepType, React.ReactNode> = {
  script: <CodeOutlined />,
  approval: <AuditOutlined />,
  condition: <BranchesOutlined />,
  wait: <ClockCircleOutlined />,
  notify: <BellOutlined />,
};

/** 步骤类型标签颜色映射 */
const STEP_TYPE_COLORS: Record<StepType, string> = {
  script: 'blue',
  approval: 'orange',
  condition: 'purple',
  wait: 'cyan',
  notify: 'green',
};

/** 通知渠道选项 */
const NOTIFY_CHANNEL_OPTIONS: Array<{ value: NotifyChannel; label: string }> = [
  { value: 'email', label: '邮件' },
  { value: 'webhook', label: 'Webhook' },
  { value: 'dingtalk', label: '钉钉' },
  { value: 'wechat', label: '企业微信' },
  { value: 'sms', label: '短信' },
];

/** 创建空步骤的工厂函数 */
function createEmptyStep(type: StepType): WorkflowStep {
  const base: WorkflowStep = { name: '', type };
  switch (type) {
    case 'script':
      return { ...base, script: '', timeout: 60 };
    case 'approval':
      return { ...base, approvers: [], timeout: 3600, approvalTimeoutAction: 'auto_reject' };
    case 'condition':
      return { ...base, condition: '', trueBranch: 0, falseBranch: 0 };
    case 'wait':
      return { ...base, waitMinutes: 5 };
    case 'notify':
      return { ...base, notifyTitle: '', notifyContent: '', notifyChannels: ['dingtalk'] };
    default:
      return base;
  }
}

const WorkflowEditor: React.FC = () => {
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const isEdit = Boolean(id);

  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [steps, setSteps] = useState<WorkflowStep[]>([]);
  const [variables, setVariables] = useState<WorkflowVariable[]>([]);
  const [triggerType, setTriggerType] = useState<TriggerType>('manual');
  const [addStepModalOpen, setAddStepModalOpen] = useState(false);

  /**
   * 编辑模式下加载工作流数据
   */
  const loadWorkflow = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const workflow = await getWorkflow(id);
      form.setFieldsValue({
        name: workflow.name,
        description: workflow.description,
        triggerType: workflow.triggerType,
        cronExpr: workflow.cronExpr,
      });
      setTriggerType(workflow.triggerType);
      setSteps(workflow.steps || []);
      setVariables(workflow.variables || []);
    } catch {
      message.error('加载工作流失败');
    } finally {
      setLoading(false);
    }
  }, [id, form]);

  useEffect(() => {
    if (isEdit) loadWorkflow();
  }, [isEdit, loadWorkflow]);

  /**
   * 添加步骤
   * @param type 步骤类型
   */
  const handleAddStep = (type: StepType) => {
    const newStep = createEmptyStep(type);
    const typeLabel = STEP_TYPE_OPTIONS.find((opt) => opt.value === type)?.label || type;
    newStep.name = `${typeLabel} ${steps.length + 1}`;
    setSteps([...steps, newStep]);
    setAddStepModalOpen(false);
  };

  /**
   * 更新步骤
   * @param index 步骤索引
   * @param updates 更新字段
   */
  const handleUpdateStep = (index: number, updates: Partial<WorkflowStep>) => {
    const newSteps = [...steps];
    newSteps[index] = { ...newSteps[index], ...updates };
    setSteps(newSteps);
  };

  /**
   * 删除步骤
   * @param index 步骤索引
   */
  const handleDeleteStep = (index: number) => {
    setSteps(steps.filter((_, i) => i !== index));
  };

  /**
   * 上移步骤
   * @param index 步骤索引
   */
  const handleMoveUp = (index: number) => {
    if (index === 0) return;
    const newSteps = [...steps];
    [newSteps[index - 1], newSteps[index]] = [newSteps[index], newSteps[index - 1]];
    setSteps(newSteps);
  };

  /**
   * 下移步骤
   * @param index 步骤索引
   */
  const handleMoveDown = (index: number) => {
    if (index === steps.length - 1) return;
    const newSteps = [...steps];
    [newSteps[index], newSteps[index + 1]] = [newSteps[index + 1], newSteps[index]];
    setSteps(newSteps);
  };

  /**
   * 添加变量行
   */
  const handleAddVariable = () => {
    setVariables([...variables, { name: '', type: 'string', defaultValue: '', description: '' }]);
  };

  /**
   * 更新变量
   * @param index 变量索引
   * @param updates 更新字段
   */
  const handleUpdateVariable = (index: number, updates: Partial<WorkflowVariable>) => {
    const newVars = [...variables];
    newVars[index] = { ...newVars[index], ...updates };
    setVariables(newVars);
  };

  /**
   * 删除变量
   * @param index 变量索引
   */
  const handleDeleteVariable = (index: number) => {
    setVariables(variables.filter((_, i) => i !== index));
  };

  /**
   * 保存工作流
   */
  const handleSave = async () => {
    try {
      const values = await form.validateFields();
      if (steps.length === 0) {
        message.warning('请至少添加一个步骤');
        return;
      }
      setSaving(true);
      const payload = {
        name: values.name,
        description: values.description || '',
        triggerType: values.triggerType,
        cronExpr: values.triggerType === 'cron' ? values.cronExpr : undefined,
        steps,
        variables,
      };
      if (isEdit && id) {
        await updateWorkflow(id, payload);
        message.success('保存成功');
      } else {
        await createWorkflow(payload);
        message.success('创建成功');
      }
      navigate('/workflows');
    } catch {
      // 表单校验失败或 API 错误
    } finally {
      setSaving(false);
    }
  };

  /**
   * 渲染步骤配置表单（根据步骤类型不同展示不同的配置项）
   * @param step 步骤数据
   * @param index 步骤索引
   */
  const renderStepConfig = (step: WorkflowStep, index: number) => {
    switch (step.type) {
      case 'script':
        return (
          <div>
            <div style={{ marginBottom: 12 }}>
              <Text strong style={{ display: 'block', marginBottom: 4 }}>脚本内容</Text>
              <TextArea
                rows={4}
                value={step.script}
                onChange={(e) => handleUpdateStep(index, { script: e.target.value })}
                placeholder="输入要执行的 Shell 脚本命令..."
                style={{ fontFamily: 'monospace' }}
              />
            </div>
            <div>
              <Text strong style={{ marginRight: 8 }}>超时时间（秒）</Text>
              <InputNumber
                min={1}
                max={86400}
                value={step.timeout}
                onChange={(value) => handleUpdateStep(index, { timeout: value || 60 })}
              />
            </div>
          </div>
        );

      case 'approval':
        return (
          <div>
            <div style={{ marginBottom: 12 }}>
              <Text strong style={{ display: 'block', marginBottom: 4 }}>审批人</Text>
              <Select
                mode="tags"
                style={{ width: '100%' }}
                value={step.approvers}
                onChange={(value) => handleUpdateStep(index, { approvers: value })}
                placeholder="输入审批人用户名，回车确认"
              />
            </div>
            <Row gutter={16}>
              <Col span={12}>
                <Text strong style={{ display: 'block', marginBottom: 4 }}>超时时间（秒）</Text>
                <InputNumber
                  min={60}
                  max={86400}
                  value={step.timeout}
                  onChange={(value) => handleUpdateStep(index, { timeout: value || 3600 })}
                  style={{ width: '100%' }}
                />
              </Col>
              <Col span={12}>
                <Text strong style={{ display: 'block', marginBottom: 4 }}>逾期动作</Text>
                <Select
                  value={step.approvalTimeoutAction}
                  onChange={(value: ApprovalTimeoutAction) => handleUpdateStep(index, { approvalTimeoutAction: value })}
                  style={{ width: '100%' }}
                  options={[
                    { value: 'auto_approve', label: '自动通过' },
                    { value: 'auto_reject', label: '自动拒绝' },
                  ]}
                />
              </Col>
            </Row>
          </div>
        );

      case 'condition':
        return (
          <div>
            <div style={{ marginBottom: 12 }}>
              <Text strong style={{ display: 'block', marginBottom: 4 }}>条件表达式</Text>
              <Input
                value={step.condition}
                onChange={(e) => handleUpdateStep(index, { condition: e.target.value })}
                placeholder="例如: ${exit_code} == 0"
                style={{ fontFamily: 'monospace' }}
              />
            </div>
            <Row gutter={16}>
              <Col span={12}>
                <Text strong style={{ display: 'block', marginBottom: 4 }}>条件为真 → 跳转步骤序号</Text>
                <InputNumber
                  min={0}
                  max={steps.length - 1}
                  value={step.trueBranch}
                  onChange={(value) => handleUpdateStep(index, { trueBranch: value || 0 })}
                  style={{ width: '100%' }}
                />
              </Col>
              <Col span={12}>
                <Text strong style={{ display: 'block', marginBottom: 4 }}>条件为假 → 跳转步骤序号</Text>
                <InputNumber
                  min={0}
                  max={steps.length - 1}
                  value={step.falseBranch}
                  onChange={(value) => handleUpdateStep(index, { falseBranch: value || 0 })}
                  style={{ width: '100%' }}
                />
              </Col>
            </Row>
          </div>
        );

      case 'wait':
        return (
          <div>
            <Text strong style={{ marginRight: 8 }}>等待时间（分钟）</Text>
            <InputNumber
              min={1}
              max={1440}
              value={step.waitMinutes}
              onChange={(value) => handleUpdateStep(index, { waitMinutes: value || 5 })}
            />
          </div>
        );

      case 'notify':
        return (
          <div>
            <div style={{ marginBottom: 12 }}>
              <Text strong style={{ display: 'block', marginBottom: 4 }}>通知标题</Text>
              <Input
                value={step.notifyTitle}
                onChange={(e) => handleUpdateStep(index, { notifyTitle: e.target.value })}
                placeholder="输入通知标题"
              />
            </div>
            <div style={{ marginBottom: 12 }}>
              <Text strong style={{ display: 'block', marginBottom: 4 }}>通知内容</Text>
              <TextArea
                rows={3}
                value={step.notifyContent}
                onChange={(e) => handleUpdateStep(index, { notifyContent: e.target.value })}
                placeholder="输入通知内容，支持 ${变量名} 引用变量"
              />
            </div>
            <div>
              <Text strong style={{ display: 'block', marginBottom: 4 }}>通知渠道</Text>
              <Select
                mode="multiple"
                style={{ width: '100%' }}
                value={step.notifyChannels}
                onChange={(value: NotifyChannel[]) => handleUpdateStep(index, { notifyChannels: value })}
                options={NOTIFY_CHANNEL_OPTIONS}
                placeholder="选择通知渠道"
              />
            </div>
          </div>
        );

      default:
        return null;
    }
  };

  /** 变量表格列定义 */
  const variableColumns = [
    {
      title: '变量名',
      dataIndex: 'name',
      key: 'name',
      width: 180,
      render: (_: unknown, __: unknown, index: number) => (
        <Input
          value={variables[index].name}
          onChange={(e) => handleUpdateVariable(index, { name: e.target.value })}
          placeholder="变量名"
          style={{ fontFamily: 'monospace' }}
        />
      ),
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 120,
      render: (_: unknown, __: unknown, index: number) => (
        <Select
          value={variables[index].type}
          onChange={(value: 'string' | 'number' | 'boolean') => handleUpdateVariable(index, { type: value })}
          style={{ width: '100%' }}
          options={[
            { value: 'string', label: '字符串' },
            { value: 'number', label: '数字' },
            { value: 'boolean', label: '布尔值' },
          ]}
        />
      ),
    },
    {
      title: '默认值',
      dataIndex: 'defaultValue',
      key: 'defaultValue',
      width: 160,
      render: (_: unknown, __: unknown, index: number) => (
        <Input
          value={variables[index].defaultValue}
          onChange={(e) => handleUpdateVariable(index, { defaultValue: e.target.value })}
          placeholder="默认值"
        />
      ),
    },
    {
      title: '说明',
      dataIndex: 'description',
      key: 'description',
      render: (_: unknown, __: unknown, index: number) => (
        <Input
          value={variables[index].description}
          onChange={(e) => handleUpdateVariable(index, { description: e.target.value })}
          placeholder="变量说明"
        />
      ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 60,
      render: (_: unknown, __: unknown, index: number) => (
        <Button
          type="link"
          danger
          size="small"
          icon={<DeleteOutlined />}
          onClick={() => handleDeleteVariable(index)}
        />
      ),
    },
  ];

  return (
    <div>
      {/* 页面标题 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={4} style={{ margin: 0 }}>
          {isEdit ? '编辑工作流' : '新建工作流'}
        </Title>
        <Space>
          <Button icon={<RollbackOutlined />} onClick={() => navigate('/workflows')}>取消</Button>
          <Button type="primary" icon={<SaveOutlined />} loading={saving} onClick={handleSave}>
            保存
          </Button>
        </Space>
      </div>

      {/* 基本信息区 */}
      <Card title="基本信息" style={{ marginBottom: 16, borderRadius: 8 }} loading={loading}>
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="name"
                label="工作流名称"
                rules={[{ required: true, message: '请输入工作流名称' }]}
              >
                <Input placeholder="输入工作流名称" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="triggerType"
                label="触发类型"
                rules={[{ required: true, message: '请选择触发类型' }]}
                initialValue="manual"
              >
                <Select
                  onChange={(value: TriggerType) => setTriggerType(value)}
                  options={[
                    { value: 'manual', label: '手动触发' },
                    { value: 'alert', label: '告警触发' },
                    { value: 'cron', label: '定时触发' },
                  ]}
                />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="description" label="描述">
            <TextArea rows={2} placeholder="描述工作流的用途和功能" />
          </Form.Item>
          {/* 定时触发时显示 Cron 表达式输入 */}
          {triggerType === 'cron' && (
            <Form.Item
              name="cronExpr"
              label="Cron 表达式"
              rules={[{ required: true, message: '请输入 Cron 表达式' }]}
              extra={
                <Space style={{ marginTop: 4 }}>
                  <Text type="secondary">常用示例：</Text>
                  <Tag style={{ cursor: 'pointer' }} onClick={() => form.setFieldValue('cronExpr', '0 0 * * *')}>每天 0:00</Tag>
                  <Tag style={{ cursor: 'pointer' }} onClick={() => form.setFieldValue('cronExpr', '0 */6 * * *')}>每 6 小时</Tag>
                  <Tag style={{ cursor: 'pointer' }} onClick={() => form.setFieldValue('cronExpr', '0 9 * * 1')}>每周一 9:00</Tag>
                  <Tag style={{ cursor: 'pointer' }} onClick={() => form.setFieldValue('cronExpr', '*/30 * * * *')}>每 30 分钟</Tag>
                </Space>
              }
            >
              <Input
                placeholder="例如: 0 0 * * * （每天零点执行）"
                style={{ fontFamily: 'monospace', width: '50%' }}
              />
            </Form.Item>
          )}
        </Form>
      </Card>

      {/* 步骤列表编辑器 */}
      <Card
        title={`步骤配置 (${steps.length} 个步骤)`}
        style={{ marginBottom: 16, borderRadius: 8 }}
        extra={
          <Button type="primary" ghost icon={<PlusOutlined />} onClick={() => setAddStepModalOpen(true)}>
            添加步骤
          </Button>
        }
      >
        {steps.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '40px 0', color: '#86909C' }}>
            暂无步骤，点击"添加步骤"开始配置工作流
          </div>
        ) : (
          <Collapse accordion>
            {steps.map((step, index) => (
              <Panel
                key={index}
                header={
                  <Space>
                    <Tag color={STEP_TYPE_COLORS[step.type]}>
                      {STEP_TYPE_ICONS[step.type]} {STEP_TYPE_OPTIONS.find((opt) => opt.value === step.type)?.label}
                    </Tag>
                    <Text>#{index + 1} {step.name}</Text>
                  </Space>
                }
                extra={
                  <Space size={4} onClick={(e) => e.stopPropagation()}>
                    <Tooltip title="上移">
                      <Button
                        type="text"
                        size="small"
                        icon={<ArrowUpOutlined />}
                        disabled={index === 0}
                        onClick={() => handleMoveUp(index)}
                      />
                    </Tooltip>
                    <Tooltip title="下移">
                      <Button
                        type="text"
                        size="small"
                        icon={<ArrowDownOutlined />}
                        disabled={index === steps.length - 1}
                        onClick={() => handleMoveDown(index)}
                      />
                    </Tooltip>
                    <Tooltip title="删除">
                      <Button
                        type="text"
                        size="small"
                        danger
                        icon={<DeleteOutlined />}
                        onClick={() => handleDeleteStep(index)}
                      />
                    </Tooltip>
                  </Space>
                }
              >
                {/* 步骤名称 */}
                <div style={{ marginBottom: 16 }}>
                  <Text strong style={{ display: 'block', marginBottom: 4 }}>步骤名称</Text>
                  <Input
                    value={step.name}
                    onChange={(e) => handleUpdateStep(index, { name: e.target.value })}
                    placeholder="输入步骤名称"
                    style={{ width: '50%' }}
                  />
                </div>
                <Divider style={{ margin: '12px 0' }} />
                {/* 步骤类型特定配置 */}
                {renderStepConfig(step, index)}
              </Panel>
            ))}
          </Collapse>
        )}
      </Card>

      {/* 变量定义区 */}
      <Card
        title="变量定义"
        style={{ marginBottom: 16, borderRadius: 8 }}
        extra={
          <Button type="primary" ghost icon={<PlusOutlined />} onClick={handleAddVariable}>
            添加变量
          </Button>
        }
      >
        {variables.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '24px 0', color: '#86909C' }}>
            暂无变量，点击"添加变量"定义工作流变量
          </div>
        ) : (
          <Table
            columns={variableColumns}
            dataSource={variables}
            rowKey={(_, index) => String(index)}
            pagination={false}
            size="small"
          />
        )}
      </Card>

      {/* 添加步骤类型选择 Modal */}
      <Modal
        title="选择步骤类型"
        open={addStepModalOpen}
        onCancel={() => setAddStepModalOpen(false)}
        footer={null}
        width={480}
        destroyOnClose
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8, marginTop: 16 }}>
          {STEP_TYPE_OPTIONS.map((opt) => (
            <Card
              key={opt.value}
              hoverable
              size="small"
              style={{ borderRadius: 8 }}
              bodyStyle={{ padding: '12px 16px' }}
              onClick={() => handleAddStep(opt.value)}
            >
              <Space>
                <span style={{ fontSize: 20, color: '#1890ff' }}>{opt.icon}</span>
                <div>
                  <div style={{ fontWeight: 600 }}>{opt.label}</div>
                  <div style={{ fontSize: 12, color: '#86909C' }}>{opt.description}</div>
                </div>
              </Space>
            </Card>
          ))}
        </div>
      </Modal>
    </div>
  );
};

export default WorkflowEditor;
