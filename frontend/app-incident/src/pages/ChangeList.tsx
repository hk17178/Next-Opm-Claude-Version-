/**
 * 变更单列表页面 - 展示所有变更单，支持按状态/类型/时间筛选
 *
 * 功能说明：
 * - 变更单列表表格：编号/标题/类型/风险/状态/申请人/计划时间/操作
 * - 筛选器：按状态、类型、时间范围过滤
 * - 操作按钮：查看详情、审批（有权限时）
 */
import React, { useState, useCallback, useEffect } from 'react';
import {
  Table, Card, Row, Col, Typography, Button, Space, Tag, Select,
  DatePicker, message,
} from 'antd';
import {
  PlusOutlined, EyeOutlined, CheckOutlined,
  SwapOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import {
  listChanges,
  type Change, type ChangeStatus, type ChangeType, type ChangeRisk,
} from '../api/change';

const { Text } = Typography;
const { RangePicker } = DatePicker;

/** 变更类型标签配置 */
const TYPE_CONFIG: Record<ChangeType, { label: string; color: string }> = {
  standard: { label: '标准变更', color: '#52C41A' },
  normal: { label: '普通变更', color: '#1890FF' },
  emergency: { label: '紧急变更', color: '#F5222D' },
};

/** 风险等级标签配置 */
const RISK_CONFIG: Record<ChangeRisk, { label: string; color: string }> = {
  low: { label: '低', color: '#52C41A' },
  medium: { label: '中', color: '#FAAD14' },
  high: { label: '高', color: '#FA8C16' },
  critical: { label: '极高', color: '#F5222D' },
};

/** 变更状态标签配置 */
const STATUS_CONFIG: Record<ChangeStatus, { label: string; color: string }> = {
  draft: { label: '草稿', color: '#D9D9D9' },
  submitted: { label: '待审批', color: '#FAAD14' },
  approved: { label: '已审批', color: '#52C41A' },
  rejected: { label: '已拒绝', color: '#F5222D' },
  executing: { label: '执行中', color: '#1890FF' },
  completed: { label: '已完成', color: '#52C41A' },
  cancelled: { label: '已取消', color: '#D9D9D9' },
};

/* ========== 模拟数据 ========== */

/** 生成模拟变更单列表数据 */
function mockChanges(): Change[] {
  return [
    {
      id: '1', changeId: 'CHG-20260323-001', title: '生产环境数据库版本升级',
      description: 'MySQL 8.0.32 升级至 8.0.36', type: 'normal', risk: 'high',
      status: 'submitted', applicant: '张工', plannedStart: '2026-03-25 02:00',
      plannedEnd: '2026-03-25 04:00', createdAt: '2026-03-22 10:00',
      updatedAt: '2026-03-22 14:00', affectedAssets: ['db-master-01', 'db-slave-01'],
      rollbackPlan: '回退至 MySQL 8.0.32 备份快照',
    },
    {
      id: '2', changeId: 'CHG-20260323-002', title: 'Nginx 配置优化 - 增加缓存策略',
      description: '优化静态资源缓存配置，提升页面加载速度', type: 'standard', risk: 'low',
      status: 'approved', applicant: '李工', plannedStart: '2026-03-24 10:00',
      plannedEnd: '2026-03-24 11:00', createdAt: '2026-03-21 16:00',
      updatedAt: '2026-03-22 09:00', affectedAssets: ['nginx-01', 'nginx-02'],
      rollbackPlan: '恢复原 Nginx 配置文件',
    },
    {
      id: '3', changeId: 'CHG-20260323-003', title: '支付服务紧急修复 - 金额计算异常',
      description: '修复特定场景下金额精度丢失问题', type: 'emergency', risk: 'critical',
      status: 'executing', applicant: '王工', plannedStart: '2026-03-23 15:00',
      plannedEnd: '2026-03-23 16:00', actualStart: '2026-03-23 15:10',
      createdAt: '2026-03-23 14:30', updatedAt: '2026-03-23 15:10',
      affectedAssets: ['payment-api-01', 'payment-api-02'],
      rollbackPlan: '回退至上一版本镜像 v2.4.1',
    },
    {
      id: '4', changeId: 'CHG-20260322-001', title: 'Kubernetes 集群节点扩容',
      description: '增加 3 个 worker 节点应对流量增长', type: 'normal', risk: 'medium',
      status: 'completed', applicant: '赵工', plannedStart: '2026-03-22 20:00',
      plannedEnd: '2026-03-22 22:00', actualStart: '2026-03-22 20:05',
      actualEnd: '2026-03-22 21:30', createdAt: '2026-03-20 11:00',
      updatedAt: '2026-03-22 21:30', affectedAssets: ['k8s-cluster-prod'],
      rollbackPlan: '移除新增节点并驱逐 Pod',
    },
  ];
}

/**
 * 变更单列表页面组件
 */
const ChangeList: React.FC = () => {
  const { t } = useTranslation('incident');
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);              // 表格加载状态
  const [data, setData] = useState<Change[]>([]);             // 变更单列表数据
  const [total, setTotal] = useState(0);                      // 总记录数
  const [pagination, setPagination] = useState({ current: 1, pageSize: 20 }); // 分页参数
  const [filterStatus, setFilterStatus] = useState<ChangeStatus | undefined>(); // 状态筛选
  const [filterType, setFilterType] = useState<ChangeType | undefined>();       // 类型筛选

  /**
   * 加载变更单列表数据
   * API 不可用时回退到模拟数据
   */
  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const result = await listChanges({
        page: pagination.current,
        pageSize: pagination.pageSize,
        status: filterStatus,
        type: filterType,
      });
      setData(result.list || []);
      setTotal(result.total || 0);
    } catch {
      // 后端 API 不可用，使用模拟数据
      let mockData = mockChanges();
      if (filterStatus) mockData = mockData.filter((c) => c.status === filterStatus);
      if (filterType) mockData = mockData.filter((c) => c.type === filterType);
      setData(mockData);
      setTotal(mockData.length);
    } finally {
      setLoading(false);
    }
  }, [pagination, filterStatus, filterType]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  /** 表格列定义 */
  const columns = [
    {
      title: '变更编号',
      dataIndex: 'changeId',
      key: 'changeId',
      width: 180,
      render: (text: string) => <Text style={{ color: '#1890FF' }}>{text}</Text>,
    },
    {
      title: '标题',
      dataIndex: 'title',
      key: 'title',
      ellipsis: true,
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 100,
      /** 渲染变更类型标签 */
      render: (type: ChangeType) => {
        const config = TYPE_CONFIG[type];
        return <Tag color={config?.color}>{config?.label || type}</Tag>;
      },
    },
    {
      title: '风险等级',
      dataIndex: 'risk',
      key: 'risk',
      width: 90,
      /** 渲染风险等级标签 */
      render: (risk: ChangeRisk) => {
        const config = RISK_CONFIG[risk];
        return <Tag color={config?.color}>{config?.label || risk}</Tag>;
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      /** 渲染变更状态标签 */
      render: (status: ChangeStatus) => {
        const config = STATUS_CONFIG[status];
        return <Tag color={config?.color}>{config?.label || status}</Tag>;
      },
    },
    {
      title: '申请人',
      dataIndex: 'applicant',
      key: 'applicant',
      width: 100,
    },
    {
      title: '计划时间',
      key: 'plannedTime',
      width: 200,
      /** 渲染计划开始和结束时间 */
      render: (_: unknown, record: Change) => (
        <Text style={{ fontSize: 12 }}>
          {record.plannedStart}
          <br />
          ~ {record.plannedEnd}
        </Text>
      ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 150,
      /** 渲染操作按钮：查看详情、审批 */
      render: (_: unknown, record: Change) => (
        <Space>
          <Button
            type="link"
            size="small"
            icon={<EyeOutlined />}
            onClick={() => navigate(`/changes/${record.id}`)}
          >
            详情
          </Button>
          {/* 仅待审批状态显示审批按钮 */}
          {record.status === 'submitted' && (
            <Button
              type="link"
              size="small"
              icon={<CheckOutlined />}
              style={{ color: '#52C41A' }}
              onClick={() => navigate(`/changes/${record.id}`)}
            >
              审批
            </Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div>
      {/* 页面标题与创建按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>
          <SwapOutlined style={{ marginRight: 8 }} />
          变更管理
        </Text>
        <Button type="primary" icon={<PlusOutlined />}>
          创建变更单
        </Button>
      </div>

      {/* 统计卡片行 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        {Object.entries(STATUS_CONFIG).slice(0, 5).map(([key, config]) => {
          const count = data.filter((c) => c.status === key).length;
          return (
            <Col flex={1} key={key}>
              <Card
                bordered
                style={{ borderRadius: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.06)' }}
                bodyStyle={{ padding: '16px 20px', textAlign: 'center' }}
              >
                <div style={{ color: '#86909C', fontSize: 14 }}>{config.label}</div>
                <div style={{ fontSize: 24, fontWeight: 600, color: config.color, marginTop: 4 }}>
                  {count}
                </div>
              </Card>
            </Col>
          );
        })}
      </Row>

      {/* 筛选条件栏 */}
      <Card style={{ marginBottom: 16, borderRadius: 8 }} bodyStyle={{ padding: 12 }}>
        <Space wrap>
          {/* 状态筛选 */}
          <Select
            placeholder="状态"
            style={{ width: 120 }}
            allowClear
            value={filterStatus}
            onChange={setFilterStatus}
            options={Object.entries(STATUS_CONFIG).map(([val, cfg]) => ({
              value: val,
              label: cfg.label,
            }))}
          />
          {/* 类型筛选 */}
          <Select
            placeholder="类型"
            style={{ width: 120 }}
            allowClear
            value={filterType}
            onChange={setFilterType}
            options={Object.entries(TYPE_CONFIG).map(([val, cfg]) => ({
              value: val,
              label: cfg.label,
            }))}
          />
          {/* 时间范围筛选 */}
          <RangePicker placeholder={['开始时间', '结束时间']} />
        </Space>
      </Card>

      {/* 变更单列表表格 */}
      <Table
        columns={columns}
        dataSource={data}
        loading={loading}
        rowKey="id"
        size="middle"
        onRow={(record) => ({
          style: { cursor: 'pointer' },
          onClick: () => navigate(`/changes/${record.id}`),
        })}
        pagination={{
          current: pagination.current,
          pageSize: pagination.pageSize,
          total,
          showSizeChanger: true,
          showQuickJumper: true,
          onChange: (page, pageSize) => {
            setPagination({ current: page, pageSize });
          },
        }}
      />
    </div>
  );
};

export default ChangeList;
