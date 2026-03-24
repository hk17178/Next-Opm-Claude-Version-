/**
 * 品牌设置页面 - 自定义系统品牌外观
 *
 * 功能模块：
 * - Logo 上传（Upload 组件，支持 PNG/SVG）
 * - 系统名称输入
 * - 主题色选择（预设色板 + 自定义颜色输入）
 * - 浏览器 Tab 标题设置
 * - Favicon 上传
 * - 预览区域（实时预览品牌配置效果）
 * - 保存/重置按钮
 */
import React, { useState, useCallback } from 'react';
import {
  Card, Row, Col, Typography, Form, Input, Upload, Button, Space, Divider,
  Radio, message, Image, Tooltip, ColorPicker,
} from 'antd';
import type { UploadFile } from 'antd';
import {
  UploadOutlined, SaveOutlined, UndoOutlined, PictureOutlined,
  EditOutlined, BgColorsOutlined, GlobalOutlined, StarOutlined,
  EyeOutlined, AppstoreOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Text, Title } = Typography;

// ==================== 类型定义 ====================

/** 品牌配置数据结构 */
interface BrandConfig {
  systemName: string;     // 系统名称
  primaryColor: string;   // 主题色
  tabTitle: string;       // 浏览器 Tab 标题
  logoUrl: string;        // Logo URL
  faviconUrl: string;     // Favicon URL
}

// ==================== 预设色板 ====================

/** 预设主题色选项 */
const presetColors = [
  { color: '#4da6ff', name: '科技蓝' },
  { color: '#2563eb', name: '深邃蓝' },
  { color: '#00e5a0', name: '活力绿' },
  { color: '#059669', name: '翡翠绿' },
  { color: '#7c3aed', name: '高贵紫' },
  { color: '#ea580c', name: '热情橙' },
  { color: '#dc2626', name: '活力红' },
  { color: '#0891b2', name: '青碧色' },
];

// ==================== Mock 默认值 ====================

/** 默认品牌配置 */
const defaultBrandConfig: BrandConfig = {
  systemName: 'OpsNexus',
  primaryColor: '#4da6ff',
  tabTitle: 'OpsNexus - 智能全栈运维平台',
  logoUrl: '',
  faviconUrl: '',
};

// ==================== 组件实现 ====================

/**
 * 品牌设置组件
 * 管理系统 Logo、名称、主题色、Tab 标题、Favicon 等品牌元素
 */
const BrandSettings: React.FC = () => {
  const { t } = useTranslation('settings');
  const [form] = Form.useForm();                                          // 表单实例
  const [brandConfig, setBrandConfig] = useState<BrandConfig>(defaultBrandConfig); // 当前品牌配置
  const [logoFileList, setLogoFileList] = useState<UploadFile[]>([]);      // Logo 文件列表
  const [faviconFileList, setFaviconFileList] = useState<UploadFile[]>([]); // Favicon 文件列表
  const [selectedColor, setSelectedColor] = useState(defaultBrandConfig.primaryColor); // 选中的主题色
  const [colorMode, setColorMode] = useState<'preset' | 'custom'>('preset'); // 颜色选择模式

  /**
   * 保存品牌配置
   * 校验表单后提交到后端
   */
  const handleSave = useCallback(() => {
    form.validateFields().then((values) => {
      const config: BrandConfig = {
        ...values,
        primaryColor: selectedColor,
        logoUrl: brandConfig.logoUrl,
        faviconUrl: brandConfig.faviconUrl,
      };
      console.log('保存品牌配置:', config);
      message.success(t('brand.saveSuccess'));
      // TODO: 对接品牌配置保存 API
    });
  }, [form, selectedColor, brandConfig, t]);

  /**
   * 重置品牌配置为默认值
   */
  const handleReset = useCallback(() => {
    form.setFieldsValue({
      systemName: defaultBrandConfig.systemName,
      tabTitle: defaultBrandConfig.tabTitle,
    });
    setSelectedColor(defaultBrandConfig.primaryColor);
    setLogoFileList([]);
    setFaviconFileList([]);
    setBrandConfig(defaultBrandConfig);
    message.info(t('brand.resetSuccess'));
  }, [form, t]);

  /**
   * 处理表单值变更，实时更新预览
   */
  const handleValuesChange = useCallback((_: any, allValues: any) => {
    setBrandConfig((prev) => ({
      ...prev,
      systemName: allValues.systemName || prev.systemName,
      tabTitle: allValues.tabTitle || prev.tabTitle,
    }));
  }, []);

  return (
    <div>
      {/* 页面标题与操作按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('brand.title')}</Text>
        <Space>
          {/* 重置按钮 */}
          <Button icon={<UndoOutlined />} onClick={handleReset}>
            {t('brand.reset')}
          </Button>
          {/* 保存按钮 */}
          <Button type="primary" icon={<SaveOutlined />} onClick={handleSave}>
            {t('brand.save')}
          </Button>
        </Space>
      </div>

      <Row gutter={16}>
        {/* 左侧：配置表单 */}
        <Col xs={24} lg={14}>
          <Card style={{ borderRadius: 8 }} title={t('brand.configTitle')}>
            <Form
              form={form}
              layout="vertical"
              initialValues={{
                systemName: defaultBrandConfig.systemName,
                tabTitle: defaultBrandConfig.tabTitle,
              }}
              onValuesChange={handleValuesChange}
            >
              {/* Logo 上传 */}
              <Form.Item label={
                <Space><PictureOutlined />{t('brand.logo.label')}</Space>
              }>
                <Upload
                  accept=".png,.svg,.jpg,.jpeg"
                  listType="picture-card"
                  fileList={logoFileList}
                  maxCount={1}
                  beforeUpload={(file) => {
                    // 文件类型校验
                    const isValid = file.type === 'image/png' || file.type === 'image/svg+xml' || file.type === 'image/jpeg';
                    if (!isValid) {
                      message.error(t('brand.logo.typeError'));
                      return Upload.LIST_IGNORE;
                    }
                    // 文件大小校验（最大 2MB）
                    const isLt2M = file.size / 1024 / 1024 < 2;
                    if (!isLt2M) {
                      message.error(t('brand.logo.sizeError'));
                      return Upload.LIST_IGNORE;
                    }
                    return false; // 阻止自动上传
                  }}
                  onChange={({ fileList }) => setLogoFileList(fileList)}
                >
                  {logoFileList.length < 1 && (
                    <div>
                      <UploadOutlined />
                      <div style={{ marginTop: 8 }}>{t('brand.logo.upload')}</div>
                    </div>
                  )}
                </Upload>
                <Text type="secondary" style={{ fontSize: 12 }}>{t('brand.logo.hint')}</Text>
              </Form.Item>

              {/* 系统名称 */}
              <Form.Item
                name="systemName"
                label={<Space><EditOutlined />{t('brand.systemName.label')}</Space>}
                rules={[{ required: true, message: t('brand.systemName.required') }]}
              >
                <Input placeholder={t('brand.systemName.placeholder')} maxLength={30} showCount />
              </Form.Item>

              <Divider />

              {/* 主题色选择 */}
              <Form.Item label={<Space><BgColorsOutlined />{t('brand.color.label')}</Space>}>
                {/* 颜色选择模式切换 */}
                <Radio.Group
                  value={colorMode}
                  onChange={(e) => setColorMode(e.target.value)}
                  style={{ marginBottom: 12 }}
                >
                  <Radio.Button value="preset">{t('brand.color.preset')}</Radio.Button>
                  <Radio.Button value="custom">{t('brand.color.custom')}</Radio.Button>
                </Radio.Group>

                {colorMode === 'preset' ? (
                  /* 预设色板 */
                  <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                    {presetColors.map((item) => (
                      <Tooltip title={item.name} key={item.color}>
                        <div
                          onClick={() => setSelectedColor(item.color)}
                          style={{
                            width: 36,
                            height: 36,
                            borderRadius: 6,
                            backgroundColor: item.color,
                            cursor: 'pointer',
                            border: selectedColor === item.color ? '3px solid #333' : '2px solid transparent',
                            boxShadow: selectedColor === item.color ? '0 0 0 2px rgba(0,0,0,0.1)' : 'none',
                            transition: 'all 0.2s',
                          }}
                        />
                      </Tooltip>
                    ))}
                  </div>
                ) : (
                  /* 自定义颜色选择器 */
                  <Space>
                    <ColorPicker
                      value={selectedColor}
                      onChange={(_, hex) => setSelectedColor(hex)}
                      showText
                    />
                    <Text type="secondary">{t('brand.color.currentValue')}: {selectedColor}</Text>
                  </Space>
                )}
              </Form.Item>

              <Divider />

              {/* 浏览器 Tab 标题 */}
              <Form.Item
                name="tabTitle"
                label={<Space><GlobalOutlined />{t('brand.tabTitle.label')}</Space>}
                rules={[{ required: true, message: t('brand.tabTitle.required') }]}
              >
                <Input placeholder={t('brand.tabTitle.placeholder')} maxLength={60} showCount />
              </Form.Item>

              {/* Favicon 上传 */}
              <Form.Item label={
                <Space><StarOutlined />{t('brand.favicon.label')}</Space>
              }>
                <Upload
                  accept=".ico,.png,.svg"
                  listType="picture-card"
                  fileList={faviconFileList}
                  maxCount={1}
                  beforeUpload={(file) => {
                    // 文件类型校验
                    const isValid =
                      file.type === 'image/x-icon' ||
                      file.type === 'image/png' ||
                      file.type === 'image/svg+xml' ||
                      file.type === 'image/vnd.microsoft.icon';
                    if (!isValid) {
                      message.error(t('brand.favicon.typeError'));
                      return Upload.LIST_IGNORE;
                    }
                    // 文件大小校验（最大 512KB）
                    const isLt512K = file.size / 1024 < 512;
                    if (!isLt512K) {
                      message.error(t('brand.favicon.sizeError'));
                      return Upload.LIST_IGNORE;
                    }
                    return false;
                  }}
                  onChange={({ fileList }) => setFaviconFileList(fileList)}
                >
                  {faviconFileList.length < 1 && (
                    <div>
                      <UploadOutlined />
                      <div style={{ marginTop: 8 }}>{t('brand.favicon.upload')}</div>
                    </div>
                  )}
                </Upload>
                <Text type="secondary" style={{ fontSize: 12 }}>{t('brand.favicon.hint')}</Text>
              </Form.Item>
            </Form>
          </Card>
        </Col>

        {/* 右侧：预览区域 */}
        <Col xs={24} lg={10}>
          <Card
            style={{ borderRadius: 8, position: 'sticky', top: 16 }}
            title={
              <Space><EyeOutlined />{t('brand.preview.title')}</Space>
            }
          >
            {/* 模拟 Header 预览 */}
            <div style={{
              background: '#0a101c',
              borderRadius: 8,
              padding: '12px 16px',
              marginBottom: 16,
            }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                {/* Logo 预览 */}
                <div style={{
                  width: 32,
                  height: 32,
                  borderRadius: 6,
                  backgroundColor: selectedColor,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                }}>
                  <AppstoreOutlined style={{ color: '#fff', fontSize: 18 }} />
                </div>
                {/* 系统名称预览 */}
                <Text style={{ color: '#b0c4de', fontSize: 16, fontWeight: 600 }}>
                  {brandConfig.systemName || 'OpsNexus'}
                </Text>
                {/* 主题色标识线 */}
                <div style={{ flex: 1 }} />
                <div style={{
                  width: 8,
                  height: 8,
                  borderRadius: '50%',
                  backgroundColor: selectedColor,
                  boxShadow: `0 0 6px ${selectedColor}`,
                }} />
              </div>
            </div>

            {/* 模拟浏览器 Tab 预览 */}
            <div style={{
              background: '#f0f0f0',
              borderRadius: '8px 8px 0 0',
              padding: '6px 12px',
              marginBottom: 16,
              display: 'flex',
              alignItems: 'center',
              gap: 8,
            }}>
              <div style={{
                width: 12,
                height: 12,
                borderRadius: '50%',
                backgroundColor: selectedColor,
              }} />
              <Text style={{ fontSize: 12, color: '#666' }} ellipsis>
                {brandConfig.tabTitle || 'OpsNexus'}
              </Text>
            </div>

            {/* 颜色样本展示 */}
            <div style={{ marginBottom: 16 }}>
              <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 8 }}>
                {t('brand.preview.colorSample')}
              </Text>
              <Space>
                <Button type="primary" style={{ backgroundColor: selectedColor, borderColor: selectedColor }}>
                  {t('brand.preview.primaryButton')}
                </Button>
                <Button style={{ color: selectedColor, borderColor: selectedColor }}>
                  {t('brand.preview.ghostButton')}
                </Button>
                <div style={{
                  padding: '4px 12px',
                  borderRadius: 4,
                  backgroundColor: `${selectedColor}20`,
                  color: selectedColor,
                  fontSize: 12,
                }}>
                  Tag
                </div>
              </Space>
            </div>

            {/* 预览说明 */}
            <Text type="secondary" style={{ fontSize: 12 }}>
              {t('brand.preview.hint')}
            </Text>
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default BrandSettings;
