/**
 * 资产列表页面 - 展示 CMDB 资产清单，支持多维度过滤、分页查询、资产详情侧边抽屉
 * 包含：顶部操作栏、过滤条件卡片、资产分级统计、数据表格、详情 Drawer
 */
import React, { useState, useEffect, useCallback } from 'react';
import {
  Table, Card, Button, Space, Select, Input, Tag, Typography, Row, Col, Dropdown,
  Drawer, Descriptions, Badge,
} from 'antd';
import { ImportOutlined, ExportOutlined, PlusOutlined, DownOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { fetchAssets, type Asset } from '../api/asset';

const { Text } = Typography;

/** 资产分级对应的颜色映射（S 最高 → D 最低） */
const GRADE_COLORS: Record<string, string> = {
  S: '#F53F3F', A: '#FF7D00', B: '#3491FA', C: '#86909C', D: '#C9CDD4',
};

/** 资产状态对应的 Badge 状态映射 */
const STATUS_MAP: Record<string, 'success' | 'warning' | 'error' | 'default'> = {
  online: 'success',       // 在线 - 绿色
  offline: 'error',        // 离线 - 红色
  maintenance: 'warning',  // 维护中 - 橙色
};

/**
 * 资产列表组件
 * - 顶部：页面标题 + 导入/导出/新建按钮
 * - 过滤栏：业务板块、资产类型、环境、地域、分级、状态、标签搜索
 * - 统计行：总数 + 各分级数量
 * - 表格：支持行选中、点击行打开详情 Drawer
 * - 批量操作：选中行后显示批量操作下拉菜单
 */
const AssetList: React.FC = () => {
  const { t } = useTranslation('cmdb');
  const [loading, setLoading] = useState(false);           // 表格加载状态
  const [data, setData] = useState<Asset[]>([]);            // 资产列表数据
  const [total, setTotal] = useState(0);                    // 数据总条数
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]); // 已选中行的 key 集合
  const [pagination, setPagination] = useState({ current: 1, pageSize: 20 }); // 分页参数
  const [drawerOpen, setDrawerOpen] = useState(false);      // 详情 Drawer 是否打开
  const [selectedAsset, setSelectedAsset] = useState<Asset | null>(null); // 当前查看的资产

  /**
   * 加载资产列表数据
   * @param page 页码（可选）
   * @param pageSize 每页条数（可选）
   */
  const loadData = useCallback(async (page?: number, pageSize?: number) => {
    setLoading(true);
    try {
      // request<T> 已自动解包 ApiResponse.data，直接获取 AssetListResult
      const result = await fetchAssets({
        page: page || pagination.current,
        pageSize: pageSize || pagination.pageSize,
      });
      setData(result.list || []);
      setTotal(result.total || 0);
    } catch {
      // API 尚未就绪
    } finally {
      setLoading(false);
    }
  }, [pagination]);

  /** 组件挂载及依赖变化时加载数据 */
  useEffect(() => {
    loadData();
  }, [loadData]);

  /**
   * 处理表格行点击 - 打开资产详情 Drawer
   * @param record 被点击的资产记录
   */
  const handleRowClick = (record: Asset) => {
    setSelectedAsset(record);
    setDrawerOpen(true);
  };

  /** 计算各资产分级的数量（基于当前页数据） */
  const gradeCounts = ['S', 'A', 'B', 'C', 'D'].reduce<Record<string, number>>((acc, g) => {
    acc[g] = data.filter((d) => d.grade === g).length;
    return acc;
  }, {});

  /** 表格列定义 */
  const columns = [
    {
      title: t('assets.column.hostname'),
      dataIndex: 'hostname',
      key: 'hostname',
      width: 160,
    },
    {
      title: t('assets.column.ip'),
      dataIndex: 'ip',
      key: 'ip',
      width: 130,
    },
    {
      title: t('assets.column.type'),
      dataIndex: 'type',
      key: 'type',
      width: 100,
    },
    {
      title: t('assets.column.business'),
      dataIndex: 'business',
      key: 'business',
      width: 120,
    },
    {
      title: t('assets.column.grade'),
      dataIndex: 'grade',
      key: 'grade',
      width: 80,
      /** 渲染资产分级标签，使用对应颜色 */
      render: (grade: string) => (
        <Tag color={GRADE_COLORS[grade]} style={{ borderRadius: 4, fontWeight: 600, border: 'none' }}>
          {grade}
        </Tag>
      ),
    },
    {
      title: t('assets.column.env'),
      dataIndex: 'env',
      key: 'env',
      width: 80,
    },
    {
      title: t('assets.column.status'),
      dataIndex: 'status',
      key: 'status',
      width: 80,
      /** 渲染状态 Badge，显示不同颜色的状态点 */
      render: (status: string) => (
        <Badge status={STATUS_MAP[status] || 'default'} text={t(`assets.status.${status}`)} />
      ),
    },
  ];

  /** 批量操作菜单项 */
  const batchMenuItems = [
    { key: 'grade', label: t('assets.batch.changeGrade') },     // 批量修改分级
    { key: 'business', label: t('assets.batch.changeBusiness') }, // 批量修改业务
    { key: 'tags', label: t('assets.batch.changeTags') },       // 批量修改标签
  ];

  return (
    <div>
      {/* 页面标题与操作按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('assets.title')}</Text>
        <Space>
          <Button icon={<ImportOutlined />}>{t('assets.import')}</Button>
          <Button icon={<ExportOutlined />}>{t('assets.export')}</Button>
          <Button type="primary" icon={<PlusOutlined />}>{t('assets.create')}</Button>
        </Space>
      </div>

      {/* 多维度过滤条件栏 */}
      <Card style={{ marginBottom: 16, borderRadius: 8 }} bodyStyle={{ padding: 12 }}>
        <Row gutter={[12, 12]}>
          <Col><Select placeholder={t('assets.filter.business')} style={{ width: 140 }} allowClear /></Col>
          <Col><Select placeholder={t('assets.filter.type')} style={{ width: 140 }} allowClear /></Col>
          <Col><Select placeholder={t('assets.filter.env')} style={{ width: 120 }} allowClear /></Col>
          <Col><Select placeholder={t('assets.filter.region')} style={{ width: 120 }} allowClear /></Col>
          <Col>
            {/* 资产分级过滤，选项为 S/A/B/C/D */}
            <Select placeholder={t('assets.filter.grade')} style={{ width: 120 }} allowClear
              options={['S', 'A', 'B', 'C', 'D'].map((g) => ({ value: g, label: g }))}
            />
          </Col>
          <Col><Select placeholder={t('assets.filter.status')} style={{ width: 120 }} allowClear /></Col>
          <Col><Input placeholder={t('assets.filter.tags')} style={{ width: 200 }} allowClear /></Col>
          <Col>
            <Space>
              <Button>{t('assets.reset')}</Button>
              <Button type="primary">{t('assets.search')}</Button>
            </Space>
          </Col>
        </Row>
      </Card>

      {/* 资产统计摘要：总数 + 各分级数量 */}
      <div style={{ marginBottom: 8, color: '#86909C', fontSize: 13 }}>
        {t('assets.total', { count: total })}
        {' '}
        {['S', 'A', 'B', 'C', 'D'].map((g) => `${g}:${gradeCounts[g] || 0}`).join('  ')}
      </div>

      {/* 资产数据表格 */}
      <Table
        columns={columns}
        dataSource={data}
        loading={loading}
        locale={{ emptyText: t('assets.noData') }}
        rowKey="hostname"
        rowSelection={{
          selectedRowKeys,
          onChange: setSelectedRowKeys,
        }}
        onRow={(record) => ({
          style: { cursor: 'pointer' },
          // 点击行打开资产详情 Drawer
          onClick: () => handleRowClick(record),
        })}
        size="middle"
        pagination={{
          current: pagination.current,
          pageSize: pagination.pageSize,
          total,
          showSizeChanger: true,
          showQuickJumper: true,
          onChange: (page, pageSize) => {
            setPagination({ current: page, pageSize });
            loadData(page, pageSize);
          },
        }}
        footer={() =>
          // 选中行时显示批量操作按钮
          selectedRowKeys.length > 0 ? (
            <Dropdown menu={{ items: batchMenuItems }}>
              <Button>
                {t('assets.batch.title')} <DownOutlined />
              </Button>
            </Dropdown>
          ) : null
        }
      />

      {/* 资产详情 Drawer - 点击表格行时展开 */}
      <Drawer
        title={t('assets.detailTitle')}
        open={drawerOpen}
        onClose={() => { setDrawerOpen(false); setSelectedAsset(null); }}
        width={480}
      >
        {selectedAsset && (
          <Descriptions column={1} bordered size="small">
            <Descriptions.Item label={t('assets.column.hostname')}>{selectedAsset.hostname}</Descriptions.Item>
            <Descriptions.Item label={t('assets.column.ip')}>{selectedAsset.ip}</Descriptions.Item>
            <Descriptions.Item label={t('assets.column.type')}>{selectedAsset.type}</Descriptions.Item>
            <Descriptions.Item label={t('assets.column.business')}>{selectedAsset.business}</Descriptions.Item>
            <Descriptions.Item label={t('assets.column.grade')}>
              <Tag color={GRADE_COLORS[selectedAsset.grade]}>{selectedAsset.grade}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label={t('assets.column.env')}>{selectedAsset.env}</Descriptions.Item>
            <Descriptions.Item label={t('assets.column.status')}>
              <Badge status={STATUS_MAP[selectedAsset.status] || 'default'} text={selectedAsset.status} />
            </Descriptions.Item>
            {/* 以下为可选字段，仅在有值时显示 */}
            {selectedAsset.os && <Descriptions.Item label={t('assets.detail.os')}>{selectedAsset.os}</Descriptions.Item>}
            {selectedAsset.region && <Descriptions.Item label={t('assets.detail.region')}>{selectedAsset.region}</Descriptions.Item>}
            {selectedAsset.group && <Descriptions.Item label={t('assets.detail.group')}>{selectedAsset.group}</Descriptions.Item>}
            {selectedAsset.tags && (
              <Descriptions.Item label={t('assets.detail.tags')}>
                {Object.entries(selectedAsset.tags).map(([k, v]) => (
                  <Tag key={k}>{k}={v}</Tag>
                ))}
              </Descriptions.Item>
            )}
          </Descriptions>
        )}
      </Drawer>
    </div>
  );
};

export default AssetList;
