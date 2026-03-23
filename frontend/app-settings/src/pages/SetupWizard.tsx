/**
 * Web 安装向导页面（P0）
 * 系统首次部署时的分步安装引导，共 6 个步骤
 *
 * 步骤流程：
 * 1. 数据库连接 - 配置 PostgreSQL 连接参数并测试连通性
 * 2. 中间件连接 - 配置 Kafka/ES/Redis 连接并逐个测试
 * 3. LDAP 配置（可跳过） - 配置 LDAP 目录服务
 * 4. 管理员账号 - 创建系统管理员用户
 * 5. 品牌配置 - 自定义系统名称和 Logo
 * 6. 完成 - 可选导入演示数据，执行初始化并展示进度
 *
 * 数据持久化：每步完成后保存到 sessionStorage，刷新页面不丢失进度
 */
import React, { useState, useEffect, useCallback, useRef } from 'react';
import {
  Card, Steps, Button, Form, Input, InputNumber, Space, Typography, message,
  Upload, Checkbox, Progress, Result, Alert, Divider, Row, Col,
} from 'antd';
import {
  DatabaseOutlined, CloudServerOutlined, TeamOutlined, UserOutlined,
  PictureOutlined, CheckCircleOutlined, UploadOutlined, LoadingOutlined,
} from '@ant-design/icons';
import type { UploadFile } from 'antd';
import {
  testDatabase, testKafka, testElasticsearch, testRedis, testLDAP,
  startSetup, getSetupProgress,
} from '../api/setup';
import type {
  DatabaseConfig, MiddlewareConfig, LDAPConfig, AdminConfig, BrandConfig,
} from '../api/setup';

const { Text, Title, Paragraph } = Typography;

/** sessionStorage 存储键名 */
const STORAGE_KEY = 'opsnexus_setup_wizard';

/** 步骤配置 */
const STEP_ITEMS = [
  { title: '数据库', icon: <DatabaseOutlined /> },
  { title: '中间件', icon: <CloudServerOutlined /> },
  { title: 'LDAP', icon: <TeamOutlined /> },
  { title: '管理员', icon: <UserOutlined /> },
  { title: '品牌', icon: <PictureOutlined /> },
  { title: '完成', icon: <CheckCircleOutlined /> },
];

/**
 * 从 sessionStorage 读取已保存的表单数据
 * @param key - 步骤对应的存储子键
 * @returns 已保存的数据或 undefined
 */
function loadStepData<T>(key: string): T | undefined {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (raw) {
      const data = JSON.parse(raw);
      return data[key] as T;
    }
  } catch {
    // 解析失败忽略
  }
  return undefined;
}

/**
 * 保存表单数据到 sessionStorage
 * @param key - 步骤对应的存储子键
 * @param value - 要保存的数据
 */
function saveStepData(key: string, value: unknown): void {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    const data = raw ? JSON.parse(raw) : {};
    data[key] = value;
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(data));
  } catch {
    // 存储失败忽略
  }
}

/**
 * 安装向导组件
 * - 使用 Ant Design Steps 组件实现分步表单
 * - 每步数据保存到 sessionStorage 防止刷新丢失
 * - 最后一步执行系统初始化并展示进度
 */
const SetupWizard: React.FC = () => {
  /** 当前步骤（0-5） */
  const [currentStep, setCurrentStep] = useState(0);
  /** 各步骤表单实例 */
  const [dbForm] = Form.useForm();
  const [mwForm] = Form.useForm();
  const [ldapForm] = Form.useForm();
  const [adminForm] = Form.useForm();
  const [brandForm] = Form.useForm();

  /** 各中间件连接测试状态 */
  const [dbTestOk, setDbTestOk] = useState(false);
  const [kafkaTestOk, setKafkaTestOk] = useState(false);
  const [esTestOk, setEsTestOk] = useState(false);
  const [redisTestOk, setRedisTestOk] = useState(false);
  const [ldapTestOk, setLdapTestOk] = useState(false);

  /** 测试中状态 */
  const [testing, setTesting] = useState<string | null>(null);

  /** 是否导入演示数据 */
  const [importDemo, setImportDemo] = useState(false);
  /** 初始化进度百分比 */
  const [initPercent, setInitPercent] = useState(0);
  /** 初始化步骤描述 */
  const [initStep, setInitStep] = useState('');
  /** 初始化是否完成 */
  const [initCompleted, setInitCompleted] = useState(false);
  /** 初始化是否出错 */
  const [initError, setInitError] = useState<string | null>(null);
  /** 初始化是否正在执行 */
  const [initializing, setInitializing] = useState(false);

  /** Logo 文件列表 */
  const [logoFile, setLogoFile] = useState<UploadFile[]>([]);

  /** 轮询定时器引用 */
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  /**
   * 页面加载时从 sessionStorage 恢复表单数据
   */
  useEffect(() => {
    const dbData = loadStepData<DatabaseConfig>('database');
    if (dbData) dbForm.setFieldsValue(dbData);

    const mwData = loadStepData<MiddlewareConfig>('middleware');
    if (mwData) mwForm.setFieldsValue(mwData);

    const ldapData = loadStepData<LDAPConfig>('ldap');
    if (ldapData) ldapForm.setFieldsValue(ldapData);

    const adminData = loadStepData<AdminConfig>('admin');
    if (adminData) adminForm.setFieldsValue(adminData);

    const brandData = loadStepData<BrandConfig>('brand');
    if (brandData) brandForm.setFieldsValue({ systemName: brandData.systemName });

    // 恢复当前步骤
    const savedStep = loadStepData<number>('currentStep');
    if (savedStep !== undefined) setCurrentStep(savedStep);
  }, [dbForm, mwForm, ldapForm, adminForm, brandForm]);

  /** 组件卸载时清理轮询定时器 */
  useEffect(() => {
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, []);

  /**
   * 测试数据库连接
   */
  const handleTestDB = useCallback(async () => {
    try {
      const values = await dbForm.validateFields();
      setTesting('db');
      const result = await testDatabase(values);
      if (result.success) {
        setDbTestOk(true);
        message.success('数据库连接成功');
      } else {
        message.error(`数据库连接失败：${result.error}`);
      }
    } catch {
      // 表单校验失败
    } finally {
      setTesting(null);
    }
  }, [dbForm]);

  /**
   * 测试 Kafka 连接
   */
  const handleTestKafka = useCallback(async () => {
    const address = mwForm.getFieldValue('kafkaBootstrap');
    if (!address) { message.warning('请输入 Kafka 地址'); return; }
    setTesting('kafka');
    try {
      const result = await testKafka(address);
      if (result.success) { setKafkaTestOk(true); message.success('Kafka 连接成功'); }
      else { message.error(`Kafka 连接失败：${result.error}`); }
    } catch { message.error('Kafka 连接测试出错'); }
    finally { setTesting(null); }
  }, [mwForm]);

  /**
   * 测试 Elasticsearch 连接
   */
  const handleTestES = useCallback(async () => {
    const address = mwForm.getFieldValue('esAddress');
    if (!address) { message.warning('请输入 ES 地址'); return; }
    setTesting('es');
    try {
      const result = await testElasticsearch(address);
      if (result.success) { setEsTestOk(true); message.success('Elasticsearch 连接成功'); }
      else { message.error(`ES 连接失败：${result.error}`); }
    } catch { message.error('ES 连接测试出错'); }
    finally { setTesting(null); }
  }, [mwForm]);

  /**
   * 测试 Redis 连接
   */
  const handleTestRedis = useCallback(async () => {
    const address = mwForm.getFieldValue('redisAddress');
    if (!address) { message.warning('请输入 Redis 地址'); return; }
    setTesting('redis');
    try {
      const result = await testRedis(address);
      if (result.success) { setRedisTestOk(true); message.success('Redis 连接成功'); }
      else { message.error(`Redis 连接失败：${result.error}`); }
    } catch { message.error('Redis 连接测试出错'); }
    finally { setTesting(null); }
  }, [mwForm]);

  /**
   * 测试 LDAP 连接
   */
  const handleTestLDAP = useCallback(async () => {
    try {
      const values = await ldapForm.validateFields();
      setTesting('ldap');
      const result = await testLDAP(values);
      if (result.success) { setLdapTestOk(true); message.success('LDAP 连接成功'); }
      else { message.error(`LDAP 连接失败：${result.error}`); }
    } catch { /* 表单校验失败 */ }
    finally { setTesting(null); }
  }, [ldapForm]);

  /**
   * 进入下一步
   * 校验当前步骤表单并保存数据到 sessionStorage
   */
  const handleNext = useCallback(async () => {
    try {
      switch (currentStep) {
        case 0: {
          const values = await dbForm.validateFields();
          saveStepData('database', values);
          break;
        }
        case 1: {
          const values = await mwForm.validateFields();
          saveStepData('middleware', values);
          break;
        }
        case 2: {
          // LDAP 可跳过，有内容时保存
          try {
            const values = await ldapForm.validateFields();
            saveStepData('ldap', values);
          } catch {
            // 表单为空允许跳过
            saveStepData('ldap', null);
          }
          break;
        }
        case 3: {
          const values = await adminForm.validateFields();
          saveStepData('admin', values);
          break;
        }
        case 4: {
          const values = await brandForm.validateFields();
          // 处理 Logo 文件
          let logoBase64: string | undefined;
          let logoFileName: string | undefined;
          if (logoFile.length > 0 && logoFile[0].originFileObj) {
            const file = logoFile[0].originFileObj;
            logoFileName = file.name;
            logoBase64 = await new Promise<string>((resolve) => {
              const reader = new FileReader();
              reader.onload = () => resolve(reader.result as string);
              reader.readAsDataURL(file);
            });
          }
          saveStepData('brand', { ...values, logoBase64, logoFileName });
          break;
        }
      }
      const nextStep = currentStep + 1;
      setCurrentStep(nextStep);
      saveStepData('currentStep', nextStep);
    } catch {
      // 表单校验失败，antd 自动提示
    }
  }, [currentStep, dbForm, mwForm, ldapForm, adminForm, brandForm, logoFile]);

  /**
   * 返回上一步
   */
  const handlePrev = useCallback(() => {
    const prevStep = currentStep - 1;
    setCurrentStep(prevStep);
    saveStepData('currentStep', prevStep);
  }, [currentStep]);

  /**
   * 开始系统初始化
   * 收集所有步骤数据提交到后端，然后轮询进度
   */
  const handleStartInit = useCallback(async () => {
    setInitializing(true);
    setInitError(null);
    setInitPercent(0);

    try {
      const database = loadStepData<DatabaseConfig>('database')!;
      const middleware = loadStepData<MiddlewareConfig>('middleware')!;
      const ldap = loadStepData<LDAPConfig>('ldap') || undefined;
      const admin = loadStepData<AdminConfig>('admin')!;
      const brand = loadStepData<BrandConfig>('brand')!;

      await startSetup({
        database,
        middleware,
        ldap: ldap || undefined,
        admin,
        brand,
        importDemoData: importDemo,
      });

      // 轮询初始化进度
      pollRef.current = setInterval(async () => {
        try {
          const progress = await getSetupProgress();
          setInitPercent(progress.percent);
          setInitStep(progress.step);
          if (progress.completed) {
            if (pollRef.current) clearInterval(pollRef.current);
            setInitCompleted(true);
            setInitializing(false);
            // 清除 sessionStorage 中的安装数据
            sessionStorage.removeItem(STORAGE_KEY);
          }
          if (progress.error) {
            if (pollRef.current) clearInterval(pollRef.current);
            setInitError(progress.error);
            setInitializing(false);
          }
        } catch {
          // 轮询出错继续重试
        }
      }, 1500);
    } catch (err) {
      setInitError(err instanceof Error ? err.message : '初始化请求失败');
      setInitializing(false);
    }
  }, [importDemo]);

  /**
   * 初始化完成后跳转到主页面
   */
  const handleGoHome = useCallback(() => {
    window.location.href = '/';
  }, []);

  /**
   * 渲染当前步骤内容
   */
  const renderStepContent = () => {
    switch (currentStep) {
      // ==================== 步骤 1：数据库连接 ====================
      case 0:
        return (
          <div>
            <Title level={5}>PostgreSQL 数据库连接</Title>
            <Paragraph type="secondary">配置系统主数据库的连接参数</Paragraph>
            <Form form={dbForm} layout="vertical" style={{ maxWidth: 500 }}>
              {/* 主机地址 */}
              <Row gutter={16}>
                <Col span={16}>
                  <Form.Item name="host" label="主机地址"
                    rules={[{ required: true, message: '请输入数据库主机地址' }]}
                  >
                    <Input placeholder="如 localhost 或 192.168.1.100" />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  {/* 端口号 */}
                  <Form.Item name="port" label="端口" initialValue={5432}
                    rules={[{ required: true, message: '请输入端口号' }]}
                  >
                    <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
              </Row>
              {/* 数据库名称 */}
              <Form.Item name="database" label="数据库名称"
                rules={[{ required: true, message: '请输入数据库名称' }]}
              >
                <Input placeholder="如 opsnexus" />
              </Form.Item>
              {/* 用户名 */}
              <Form.Item name="username" label="用户名"
                rules={[{ required: true, message: '请输入数据库用户名' }]}
              >
                <Input placeholder="如 postgres" />
              </Form.Item>
              {/* 密码 */}
              <Form.Item name="password" label="密码"
                rules={[{ required: true, message: '请输入数据库密码' }]}
              >
                <Input.Password placeholder="数据库密码" />
              </Form.Item>
              {/* 测试连接按钮 */}
              <Button
                loading={testing === 'db'}
                onClick={handleTestDB}
                icon={dbTestOk ? <CheckCircleOutlined style={{ color: '#00B42A' }} /> : undefined}
              >
                {dbTestOk ? '连接成功' : '测试连接'}
              </Button>
            </Form>
          </div>
        );

      // ==================== 步骤 2：中间件连接 ====================
      case 1:
        return (
          <div>
            <Title level={5}>中间件连接配置</Title>
            <Paragraph type="secondary">配置 Kafka、Elasticsearch、Redis 的连接地址</Paragraph>
            <Form form={mwForm} layout="vertical" style={{ maxWidth: 500 }}>
              {/* Kafka */}
              <Form.Item name="kafkaBootstrap" label="Kafka Bootstrap 地址"
                rules={[{ required: true, message: '请输入 Kafka 地址' }]}
              >
                <Input placeholder="如 localhost:9092" />
              </Form.Item>
              <Button
                loading={testing === 'kafka'}
                onClick={handleTestKafka}
                icon={kafkaTestOk ? <CheckCircleOutlined style={{ color: '#00B42A' }} /> : undefined}
                style={{ marginBottom: 16 }}
              >
                {kafkaTestOk ? 'Kafka 连接成功' : '测试 Kafka 连接'}
              </Button>

              <Divider />
              {/* Elasticsearch */}
              <Form.Item name="esAddress" label="Elasticsearch 地址"
                rules={[{ required: true, message: '请输入 ES 地址' }]}
              >
                <Input placeholder="如 http://localhost:9200" />
              </Form.Item>
              <Button
                loading={testing === 'es'}
                onClick={handleTestES}
                icon={esTestOk ? <CheckCircleOutlined style={{ color: '#00B42A' }} /> : undefined}
                style={{ marginBottom: 16 }}
              >
                {esTestOk ? 'ES 连接成功' : '测试 ES 连接'}
              </Button>

              <Divider />
              {/* Redis */}
              <Form.Item name="redisAddress" label="Redis 地址"
                rules={[{ required: true, message: '请输入 Redis 地址' }]}
              >
                <Input placeholder="如 localhost:6379" />
              </Form.Item>
              <Button
                loading={testing === 'redis'}
                onClick={handleTestRedis}
                icon={redisTestOk ? <CheckCircleOutlined style={{ color: '#00B42A' }} /> : undefined}
              >
                {redisTestOk ? 'Redis 连接成功' : '测试 Redis 连接'}
              </Button>
            </Form>
          </div>
        );

      // ==================== 步骤 3：LDAP 配置（可跳过） ====================
      case 2:
        return (
          <div>
            <Title level={5}>LDAP 目录服务配置</Title>
            <Paragraph type="secondary">
              配置 LDAP 以实现统一身份认证（可选，跳过则仅使用本地账号）
            </Paragraph>
            <Alert
              type="info"
              showIcon
              message='此步骤为可选配置，如不需要 LDAP 集成可直接点击"下一步"跳过。'
              style={{ marginBottom: 16 }}
            />
            <Form form={ldapForm} layout="vertical" style={{ maxWidth: 500 }}>
              {/* LDAP 服务器地址 */}
              <Form.Item name="serverURL" label="LDAP 服务器地址">
                <Input placeholder="如 ldap://ldap.example.com:389" />
              </Form.Item>
              {/* Base DN */}
              <Form.Item name="baseDN" label="Base DN">
                <Input placeholder="如 dc=example,dc=com" />
              </Form.Item>
              {/* 绑定账号 */}
              <Form.Item name="bindDN" label="绑定账号 (Bind DN)">
                <Input placeholder="如 cn=admin,dc=example,dc=com" />
              </Form.Item>
              {/* 绑定密码 */}
              <Form.Item name="bindPassword" label="绑定密码">
                <Input.Password placeholder="LDAP 绑定密码" />
              </Form.Item>
              {/* 测试 LDAP 连接 */}
              <Button
                loading={testing === 'ldap'}
                onClick={handleTestLDAP}
                icon={ldapTestOk ? <CheckCircleOutlined style={{ color: '#00B42A' }} /> : undefined}
              >
                {ldapTestOk ? 'LDAP 连接成功' : '测试 LDAP 连接'}
              </Button>
            </Form>
          </div>
        );

      // ==================== 步骤 4：管理员账号 ====================
      case 3:
        return (
          <div>
            <Title level={5}>创建管理员账号</Title>
            <Paragraph type="secondary">设置系统初始管理员的登录凭据</Paragraph>
            <Form form={adminForm} layout="vertical" style={{ maxWidth: 500 }}>
              {/* 用户名 */}
              <Form.Item name="username" label="用户名"
                rules={[{ required: true, message: '请输入管理员用户名' }]}
              >
                <Input placeholder="如 admin" />
              </Form.Item>
              {/* 密码 */}
              <Form.Item name="password" label="密码"
                rules={[
                  { required: true, message: '请输入密码' },
                  { min: 8, message: '密码长度至少 8 位' },
                ]}
              >
                <Input.Password placeholder="至少 8 位，建议包含大小写字母和数字" />
              </Form.Item>
              {/* 确认密码 */}
              <Form.Item
                name="confirmPassword"
                label="确认密码"
                dependencies={['password']}
                rules={[
                  { required: true, message: '请再次输入密码' },
                  ({ getFieldValue }) => ({
                    validator(_, value) {
                      if (!value || getFieldValue('password') === value) {
                        return Promise.resolve();
                      }
                      return Promise.reject(new Error('两次输入的密码不一致'));
                    },
                  }),
                ]}
              >
                <Input.Password placeholder="再次输入密码" />
              </Form.Item>
              {/* 邮箱 */}
              <Form.Item name="email" label="邮箱"
                rules={[
                  { required: true, message: '请输入邮箱' },
                  { type: 'email', message: '请输入有效的邮箱地址' },
                ]}
              >
                <Input placeholder="admin@example.com" />
              </Form.Item>
            </Form>
          </div>
        );

      // ==================== 步骤 5：品牌配置 ====================
      case 4:
        return (
          <div>
            <Title level={5}>品牌配置</Title>
            <Paragraph type="secondary">自定义系统名称和 Logo</Paragraph>
            <Form form={brandForm} layout="vertical" style={{ maxWidth: 500 }}>
              {/* 系统名称 */}
              <Form.Item name="systemName" label="系统名称" initialValue="OpsNexus"
                rules={[{ required: true, message: '请输入系统名称' }]}
              >
                <Input placeholder="如 OpsNexus" />
              </Form.Item>
              {/* Logo 上传 */}
              <Form.Item label="系统 Logo">
                <Upload
                  accept=".png,.svg"
                  maxCount={1}
                  fileList={logoFile}
                  onChange={({ fileList }) => setLogoFile(fileList)}
                  beforeUpload={() => false}
                  listType="picture"
                >
                  <Button icon={<UploadOutlined />}>上传 Logo（PNG/SVG）</Button>
                </Upload>
              </Form.Item>
            </Form>
          </div>
        );

      // ==================== 步骤 6：完成 ====================
      case 5:
        return (
          <div>
            {/* 初始化未开始 */}
            {!initializing && !initCompleted && !initError && (
              <div>
                <Result
                  icon={<CheckCircleOutlined style={{ color: '#00B42A' }} />}
                  title="配置已完成"
                  subTitle="所有配置项已填写完毕，点击下方按钮开始系统初始化。"
                />
                <div style={{ textAlign: 'center', marginBottom: 24 }}>
                  <Checkbox
                    checked={importDemo}
                    onChange={(e) => setImportDemo(e.target.checked)}
                  >
                    导入演示数据（包含示例告警、指标和仪表盘）
                  </Checkbox>
                </div>
                <div style={{ textAlign: 'center' }}>
                  <Button type="primary" size="large" onClick={handleStartInit}>
                    开始初始化
                  </Button>
                </div>
              </div>
            )}

            {/* 初始化进行中 */}
            {initializing && (
              <div style={{ textAlign: 'center', padding: '48px 0' }}>
                <LoadingOutlined style={{ fontSize: 48, color: '#1677FF', marginBottom: 24 }} />
                <Title level={4}>正在初始化系统...</Title>
                <Paragraph type="secondary">{initStep || '准备中...'}</Paragraph>
                <Progress percent={initPercent} status="active" style={{ maxWidth: 400, margin: '0 auto' }} />
              </div>
            )}

            {/* 初始化完成 */}
            {initCompleted && (
              <Result
                status="success"
                title="系统初始化完成"
                subTitle="OpsNexus 已准备就绪，点击下方按钮进入系统。"
                extra={
                  <Button type="primary" size="large" onClick={handleGoHome}>
                    进入系统
                  </Button>
                }
              />
            )}

            {/* 初始化出错 */}
            {initError && (
              <Result
                status="error"
                title="初始化失败"
                subTitle={initError}
                extra={
                  <Space>
                    <Button onClick={() => { setInitError(null); setCurrentStep(0); }}>
                      返回检查配置
                    </Button>
                    <Button type="primary" onClick={handleStartInit}>
                      重试
                    </Button>
                  </Space>
                }
              />
            )}
          </div>
        );

      default:
        return null;
    }
  };

  return (
    <div style={{ maxWidth: 900, margin: '0 auto', padding: '24px 0' }}>
      {/* 页面标题 */}
      <div style={{ textAlign: 'center', marginBottom: 32 }}>
        <Title level={3}>OpsNexus 系统安装向导</Title>
        <Paragraph type="secondary">请按步骤完成系统初始配置</Paragraph>
      </div>

      {/* 步骤指示器 */}
      <Steps
        current={currentStep}
        items={STEP_ITEMS}
        style={{ marginBottom: 32 }}
      />

      {/* 步骤内容区域 */}
      <Card style={{ borderRadius: 8, minHeight: 400 }}>
        {renderStepContent()}
      </Card>

      {/* 底部导航按钮 */}
      {currentStep < 5 && (
        <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 16 }}>
          <Button
            disabled={currentStep === 0}
            onClick={handlePrev}
          >
            上一步
          </Button>
          <Button type="primary" onClick={handleNext}>
            {currentStep === 2 ? '下一步（可跳过）' : '下一步'}
          </Button>
        </div>
      )}
      {/* 最后一步只显示上一步按钮（未开始初始化时） */}
      {currentStep === 5 && !initializing && !initCompleted && (
        <div style={{ marginTop: 16 }}>
          <Button onClick={handlePrev}>
            上一步
          </Button>
        </div>
      )}
    </div>
  );
};

export default SetupWizard;
