/**
 * 系统配置页面 - 管理 AI 模型参数和告警阈值配置
 * 包含两个 Tab：AI 配置（模型选择、推理参数）、告警配置（阈值、降噪、升级策略）
 */
import React, { useState } from 'react';
import {
  Card, Row, Col, Typography, Tabs, Form, Input, InputNumber, Select, Switch, Button, Space, Divider,
} from 'antd';
import { SaveOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Text, Title } = Typography;

/**
 * 系统配置组件
 * - AI 配置 Tab：默认/备选模型选择、Temperature/MaxTokens/超时等推理参数、AI 分析开关
 * - 告警配置 Tab：CPU/内存/磁盘/延迟阈值、降噪开关/收敛窗口/自动恢复、P0/P1 升级时间
 */
const SystemConfig: React.FC = () => {
  const { t } = useTranslation('settings');
  const [aiForm] = Form.useForm();       // AI 配置表单实例
  const [alertForm] = Form.useForm();     // 告警配置表单实例
  const [activeTab, setActiveTab] = useState('ai'); // 当前活动 Tab

  /**
   * 保存 AI 配置
   * 校验表单后调用 API 保存（当前为占位实现）
   */
  const handleSaveAI = () => {
    aiForm.validateFields().then(() => {
      // TODO: 对接系统配置保存 API
    });
  };

  /**
   * 保存告警配置
   * 校验表单后调用 API 保存（当前为占位实现）
   */
  const handleSaveAlert = () => {
    alertForm.validateFields().then(() => {
      // TODO: 对接系统配置保存 API
    });
  };

  /** Tab 配置项 */
  const tabItems = [
    {
      key: 'ai',
      label: t('sysConfig.tab.ai'),
      children: (
        <Form form={aiForm} layout="vertical" style={{ maxWidth: 600 }}>
          {/* AI 模型选择区域 */}
          <Title level={5}>{t('sysConfig.ai.defaultModel')}</Title>
          {/* 默认模型选择 */}
          <Form.Item name="defaultModel" label={t('sysConfig.ai.modelLabel')}>
            <Select
              placeholder={t('sysConfig.ai.modelPlaceholder')}
              options={[
                { value: 'claude-sonnet', label: 'Claude Sonnet' },
                { value: 'qwen-plus', label: 'Qwen Plus' },
                { value: 'deepseek-chat', label: 'DeepSeek Chat' },
              ]}
            />
          </Form.Item>
          {/* 备选模型选择（主模型不可用时自动切换） */}
          <Form.Item name="fallbackModel" label={t('sysConfig.ai.fallbackLabel')}>
            <Select
              placeholder={t('sysConfig.ai.fallbackPlaceholder')}
              options={[
                { value: 'claude-sonnet', label: 'Claude Sonnet' },
                { value: 'qwen-plus', label: 'Qwen Plus' },
                { value: 'deepseek-chat', label: 'DeepSeek Chat' },
              ]}
            />
          </Form.Item>

          <Divider />
          {/* 推理参数配置区域 */}
          <Title level={5}>{t('sysConfig.ai.params')}</Title>
          <Row gutter={16}>
            <Col span={12}>
              {/* Temperature：控制输出随机性，0 最确定，2 最随机 */}
              <Form.Item name="temperature" label={t('sysConfig.ai.temperature')}>
                <InputNumber min={0} max={2} step={0.1} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              {/* 最大 Token 数：控制单次推理的输出长度上限 */}
              <Form.Item name="maxTokens" label={t('sysConfig.ai.maxTokens')}>
                <InputNumber min={100} max={100000} step={100} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>
          {/* 推理超时时间（秒） */}
          <Form.Item name="timeout" label={t('sysConfig.ai.timeout')}>
            <InputNumber min={5} max={300} addonAfter="s" style={{ width: 200 }} />
          </Form.Item>
          {/* AI 智能分析总开关 */}
          <Form.Item name="enableAIAnalysis" label={t('sysConfig.ai.enableAnalysis')} valuePropName="checked">
            <Switch />
          </Form.Item>

          <Form.Item>
            <Button type="primary" icon={<SaveOutlined />} onClick={handleSaveAI}>
              {t('sysConfig.save')}
            </Button>
          </Form.Item>
        </Form>
      ),
    },
    {
      key: 'alert',
      label: t('sysConfig.tab.alert'),
      children: (
        <Form form={alertForm} layout="vertical" style={{ maxWidth: 600 }}>
          {/* 告警阈值配置区域 */}
          <Title level={5}>{t('sysConfig.alert.thresholds')}</Title>
          <Row gutter={16}>
            <Col span={12}>
              {/* CPU 使用率告警阈值 */}
              <Form.Item name="cpuThreshold" label={t('sysConfig.alert.cpuThreshold')}>
                <InputNumber min={0} max={100} addonAfter="%" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              {/* 内存使用率告警阈值 */}
              <Form.Item name="memoryThreshold" label={t('sysConfig.alert.memoryThreshold')}>
                <InputNumber min={0} max={100} addonAfter="%" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}>
              {/* 磁盘使用率告警阈值 */}
              <Form.Item name="diskThreshold" label={t('sysConfig.alert.diskThreshold')}>
                <InputNumber min={0} max={100} addonAfter="%" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              {/* 请求延迟告警阈值（毫秒） */}
              <Form.Item name="latencyThreshold" label={t('sysConfig.alert.latencyThreshold')}>
                <InputNumber min={0} max={60000} addonAfter="ms" style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          <Divider />
          {/* 降噪策略配置区域 */}
          <Title level={5}>{t('sysConfig.alert.noiseReduction')}</Title>
          {/* 降噪总开关 */}
          <Form.Item name="enableNoiseReduction" label={t('sysConfig.alert.enableNR')} valuePropName="checked">
            <Switch />
          </Form.Item>
          {/* 告警收敛窗口：在窗口期内相同告警只计一次 */}
          <Form.Item name="convergenceWindow" label={t('sysConfig.alert.convergenceWindow')}>
            <InputNumber min={1} max={60} addonAfter="min" style={{ width: 200 }} />
          </Form.Item>
          {/* 自动恢复超时：超过此时间未恢复则自动关闭告警 */}
          <Form.Item name="autoResolveTimeout" label={t('sysConfig.alert.autoResolve')}>
            <InputNumber min={1} max={1440} addonAfter="min" style={{ width: 200 }} />
          </Form.Item>

          <Divider />
          {/* 升级策略配置区域 */}
          <Title level={5}>{t('sysConfig.alert.escalation')}</Title>
          {/* P0 事件未响应时自动升级时间 */}
          <Form.Item name="p0EscalationTime" label={t('sysConfig.alert.p0Escalation')}>
            <InputNumber min={1} max={60} addonAfter="min" style={{ width: 200 }} />
          </Form.Item>
          {/* P1 事件未响应时自动升级时间 */}
          <Form.Item name="p1EscalationTime" label={t('sysConfig.alert.p1Escalation')}>
            <InputNumber min={1} max={120} addonAfter="min" style={{ width: 200 }} />
          </Form.Item>

          <Form.Item>
            <Button type="primary" icon={<SaveOutlined />} onClick={handleSaveAlert}>
              {t('sysConfig.save')}
            </Button>
          </Form.Item>
        </Form>
      ),
    },
  ];

  return (
    <div>
      {/* 页面标题 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('sysConfig.title')}</Text>
      </div>

      <Card style={{ borderRadius: 8 }}>
        <Tabs items={tabItems} activeKey={activeTab} onChange={setActiveTab} />
      </Card>
    </div>
  );
};

export default SystemConfig;
