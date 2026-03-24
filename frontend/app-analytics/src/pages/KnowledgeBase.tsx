/**
 * 知识库页面
 * 路由: /analytics/knowledge
 *
 * 功能模块：
 * - 知识分类树（左侧 Tree：故障处理/运维手册/最佳实践/FAQ）
 * - 知识列表（标题/分类/作者/更新时间/浏览量/点赞数）
 * - 搜索栏（关键词搜索 + 分类过滤）
 * - 知识详情 Drawer（Markdown 内容渲染）
 * - 创建/编辑知识弹窗（标题+分类+内容编辑器）
 * - 热门知识标签云
 *
 * 数据来源：Mock 数据（后端就绪后替换）
 */
import React, { useState, useMemo } from 'react';
import {
  Card, Row, Col, Tree, Table, Input, Select, Button, Drawer, Modal,
  Form, Tag, Space, Typography, Badge, Tooltip,
} from 'antd';
import type { DataNode } from 'antd/es/tree';
import {
  SearchOutlined, PlusOutlined, EyeOutlined, LikeOutlined,
  LikeFilled, EditOutlined, DeleteOutlined,
  BookOutlined, ToolOutlined, BulbOutlined, QuestionCircleOutlined,
  FolderOutlined, FileTextOutlined, TagsOutlined,
  ClockCircleOutlined, UserOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Text, Title, Paragraph } = Typography;
const { Search } = Input;
const { TextArea } = Input;

/* ==================== 类型定义 ==================== */

/** 知识条目 */
interface KnowledgeItem {
  key: string;
  title: string;
  category: string;
  categoryKey: string;
  author: string;
  updatedAt: string;
  views: number;
  likes: number;
  tags: string[];
  content: string;
}

/* ==================== Mock 数据 ==================== */

/** 知识分类树数据 */
const CATEGORY_TREE: DataNode[] = [
  {
    key: 'all',
    title: '全部知识',
    icon: <FolderOutlined />,
    children: [
      {
        key: 'fault-handling',
        title: '故障处理',
        icon: <BookOutlined style={{ color: '#ff6b6b' }} />,
        children: [
          { key: 'fault-db', title: '数据库故障', icon: <FileTextOutlined />, isLeaf: true },
          { key: 'fault-network', title: '网络故障', icon: <FileTextOutlined />, isLeaf: true },
          { key: 'fault-app', title: '应用故障', icon: <FileTextOutlined />, isLeaf: true },
        ],
      },
      {
        key: 'ops-manual',
        title: '运维手册',
        icon: <ToolOutlined style={{ color: '#4da6ff' }} />,
        children: [
          { key: 'manual-deploy', title: '部署手册', icon: <FileTextOutlined />, isLeaf: true },
          { key: 'manual-monitor', title: '监控配置', icon: <FileTextOutlined />, isLeaf: true },
          { key: 'manual-backup', title: '备份恢复', icon: <FileTextOutlined />, isLeaf: true },
        ],
      },
      {
        key: 'best-practice',
        title: '最佳实践',
        icon: <BulbOutlined style={{ color: '#00e5a0' }} />,
        children: [
          { key: 'bp-ha', title: '高可用架构', icon: <FileTextOutlined />, isLeaf: true },
          { key: 'bp-perf', title: '性能优化', icon: <FileTextOutlined />, isLeaf: true },
        ],
      },
      {
        key: 'faq',
        title: 'FAQ',
        icon: <QuestionCircleOutlined style={{ color: '#ffaa33' }} />,
        children: [
          { key: 'faq-common', title: '常见问题', icon: <FileTextOutlined />, isLeaf: true },
          { key: 'faq-tools', title: '工具使用', icon: <FileTextOutlined />, isLeaf: true },
        ],
      },
    ],
  },
];

/** 知识条目 Mock 数据 */
const MOCK_KNOWLEDGE: KnowledgeItem[] = [
  {
    key: '1',
    title: 'MySQL 主从复制延迟排查指南',
    category: '故障处理',
    categoryKey: 'fault-db',
    author: '张运维',
    updatedAt: '2026-03-24 10:30',
    views: 1280,
    likes: 86,
    tags: ['MySQL', '主从复制', '延迟'],
    content: `# MySQL 主从复制延迟排查指南

## 问题描述
MySQL 主从复制延迟是常见的数据库问题，可能导致数据不一致。

## 排查步骤

### 1. 检查从库状态
\`\`\`sql
SHOW SLAVE STATUS\\G
\`\`\`
关注 \`Seconds_Behind_Master\` 字段。

### 2. 检查主库 binlog
\`\`\`sql
SHOW MASTER STATUS;
SHOW BINARY LOGS;
\`\`\`

### 3. 常见原因
- 从库硬件性能不足
- 大事务导致回放缓慢
- 网络带宽不足
- 表缺少主键导致行查找缓慢

### 4. 解决方案
1. 启用并行复制 (slave_parallel_workers)
2. 优化大事务，拆分批量操作
3. 确保从库硬件配置不低于主库
4. 为所有表添加主键`,
  },
  {
    key: '2',
    title: 'Kubernetes Pod 异常重启排查手册',
    category: '故障处理',
    categoryKey: 'fault-app',
    author: '李工',
    updatedAt: '2026-03-23 15:20',
    views: 956,
    likes: 72,
    tags: ['K8s', 'Pod', '重启'],
    content: `# Kubernetes Pod 异常重启排查手册

## 常见原因
1. OOMKilled - 内存超限
2. CrashLoopBackOff - 应用启动失败
3. Liveness probe 失败
4. 资源配额不足

## 排查命令
\`\`\`bash
kubectl describe pod <pod-name>
kubectl logs <pod-name> --previous
kubectl top pod <pod-name>
\`\`\``,
  },
  {
    key: '3',
    title: 'Nginx 反向代理配置最佳实践',
    category: '运维手册',
    categoryKey: 'manual-deploy',
    author: '王架构',
    updatedAt: '2026-03-22 09:15',
    views: 2340,
    likes: 145,
    tags: ['Nginx', '反向代理', '配置'],
    content: `# Nginx 反向代理配置最佳实践

## 基础配置模板
\`\`\`nginx
upstream backend {
    server 10.0.0.1:8080 weight=3;
    server 10.0.0.2:8080 weight=2;
    keepalive 32;
}
\`\`\`

## 关键优化项
- 开启 keepalive 连接复用
- 配置合理的超时时间
- 启用 gzip 压缩
- 设置缓冲区大小`,
  },
  {
    key: '4',
    title: 'Prometheus 告警规则配置指南',
    category: '运维手册',
    categoryKey: 'manual-monitor',
    author: '赵监控',
    updatedAt: '2026-03-21 14:00',
    views: 1890,
    likes: 98,
    tags: ['Prometheus', '告警', '监控'],
    content: `# Prometheus 告警规则配置指南

## 告警规则模板
\`\`\`yaml
groups:
- name: 基础设施告警
  rules:
  - alert: CPU使用率过高
    expr: node_cpu_usage > 85
    for: 5m
    labels:
      severity: warning
\`\`\``,
  },
  {
    key: '5',
    title: '微服务高可用架构设计要点',
    category: '最佳实践',
    categoryKey: 'bp-ha',
    author: '钱架构师',
    updatedAt: '2026-03-20 11:30',
    views: 3120,
    likes: 210,
    tags: ['微服务', '高可用', '架构'],
    content: `# 微服务高可用架构设计要点

## 核心原则
1. 服务无状态化
2. 数据库读写分离
3. 缓存多级策略
4. 优雅降级与熔断
5. 异步消息解耦`,
  },
  {
    key: '6',
    title: '生产环境数据库备份恢复流程',
    category: '运维手册',
    categoryKey: 'manual-backup',
    author: '孙DBA',
    updatedAt: '2026-03-19 16:45',
    views: 1560,
    likes: 88,
    tags: ['备份', '恢复', '数据库'],
    content: `# 生产环境数据库备份恢复流程

## 备份策略
- 全量备份：每日凌晨 2:00
- 增量备份：每 4 小时
- binlog 实时归档

## 恢复步骤
1. 确认恢复目标时间点
2. 还原最近全量备份
3. 应用增量备份
4. 回放 binlog 至目标时间点`,
  },
  {
    key: '7',
    title: 'JVM 性能调优实战总结',
    category: '最佳实践',
    categoryKey: 'bp-perf',
    author: '周开发',
    updatedAt: '2026-03-18 10:20',
    views: 2780,
    likes: 167,
    tags: ['JVM', '性能', 'GC'],
    content: `# JVM 性能调优实战总结

## 调优目标
- 降低 GC 停顿时间
- 提高吞吐量
- 减少内存占用

## 关键参数
\`\`\`
-Xms4g -Xmx4g
-XX:+UseG1GC
-XX:MaxGCPauseMillis=200
\`\`\``,
  },
  {
    key: '8',
    title: '常见 HTTP 状态码及排查思路',
    category: 'FAQ',
    categoryKey: 'faq-common',
    author: '吴运维',
    updatedAt: '2026-03-17 09:00',
    views: 4520,
    likes: 320,
    tags: ['HTTP', '状态码', 'FAQ'],
    content: `# 常见 HTTP 状态码及排查思路

## 4xx 客户端错误
- **400** Bad Request：请求参数格式错误
- **401** Unauthorized：认证失败
- **403** Forbidden：权限不足
- **404** Not Found：资源不存在

## 5xx 服务端错误
- **500** Internal Server Error：应用内部异常
- **502** Bad Gateway：上游服务不可达
- **503** Service Unavailable：服务过载
- **504** Gateway Timeout：上游响应超时`,
  },
];

/** 热门标签云数据 */
const HOT_TAGS = [
  { name: 'MySQL', count: 45, color: '#4da6ff' },
  { name: 'Kubernetes', count: 38, color: '#00e5a0' },
  { name: 'Nginx', count: 32, color: '#ffaa33' },
  { name: 'Redis', count: 28, color: '#ff6b6b' },
  { name: '监控', count: 25, color: '#6366f1' },
  { name: '高可用', count: 22, color: '#4da6ff' },
  { name: '性能优化', count: 20, color: '#00e5a0' },
  { name: 'Docker', count: 18, color: '#ffaa33' },
  { name: '安全', count: 15, color: '#ff6b6b' },
  { name: '备份恢复', count: 14, color: '#6366f1' },
  { name: 'JVM', count: 12, color: '#4da6ff' },
  { name: 'Prometheus', count: 11, color: '#00e5a0' },
];

/* ==================== 主组件 ==================== */

/**
 * 知识库页面组件
 * 提供运维知识的分类浏览、搜索、查看详情、创建编辑等功能
 */
const KnowledgeBase: React.FC = () => {
  const { t } = useTranslation('analytics');
  const [form] = Form.useForm();

  /* ---------- 状态管理 ---------- */
  /** 搜索关键词 */
  const [searchText, setSearchText] = useState('');
  /** 当前选中的分类 key */
  const [selectedCategory, setSelectedCategory] = useState<string>('all');
  /** 分类过滤下拉值 */
  const [categoryFilter, setCategoryFilter] = useState<string | undefined>(undefined);
  /** 知识详情 Drawer 可见性 */
  const [drawerVisible, setDrawerVisible] = useState(false);
  /** 当前查看的知识条目 */
  const [currentItem, setCurrentItem] = useState<KnowledgeItem | null>(null);
  /** 创建/编辑弹窗可见性 */
  const [modalVisible, setModalVisible] = useState(false);
  /** 编辑模式（true=编辑, false=创建） */
  const [isEditing, setIsEditing] = useState(false);
  /** 点赞记录（记录已点赞的知识 key） */
  const [likedKeys, setLikedKeys] = useState<Set<string>>(new Set());

  /* ---------- 数据过滤 ---------- */

  /** 根据搜索和分类条件过滤知识列表 */
  const filteredKnowledge = useMemo(() => {
    return MOCK_KNOWLEDGE.filter((item) => {
      // 关键词过滤（标题、标签、内容）
      const matchSearch = !searchText
        || item.title.toLowerCase().includes(searchText.toLowerCase())
        || item.tags.some((tag) => tag.toLowerCase().includes(searchText.toLowerCase()));

      // 分类过滤（支持树选择和下拉选择两种方式）
      const activeCategory = categoryFilter || selectedCategory;
      const matchCategory = activeCategory === 'all'
        || item.categoryKey === activeCategory
        || item.categoryKey.startsWith(activeCategory);

      return matchSearch && matchCategory;
    });
  }, [searchText, selectedCategory, categoryFilter]);

  /* ---------- 事件处理 ---------- */

  /** 选择分类树节点 */
  const handleCategorySelect = (keys: React.Key[]) => {
    if (keys.length > 0) {
      setSelectedCategory(keys[0] as string);
      setCategoryFilter(undefined); // 清空下拉过滤
    }
  };

  /** 打开知识详情 Drawer */
  const handleViewDetail = (item: KnowledgeItem) => {
    setCurrentItem(item);
    setDrawerVisible(true);
  };

  /** 打开创建弹窗 */
  const handleCreate = () => {
    setIsEditing(false);
    form.resetFields();
    setModalVisible(true);
  };

  /** 打开编辑弹窗 */
  const handleEdit = (item: KnowledgeItem) => {
    setIsEditing(true);
    form.setFieldsValue({
      title: item.title,
      category: item.categoryKey,
      content: item.content,
      tags: item.tags.join(', '),
    });
    setModalVisible(true);
  };

  /** 处理点赞 */
  const handleLike = (key: string) => {
    setLikedKeys((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  /** 保存知识（创建或编辑） */
  const handleSave = () => {
    form.validateFields().then(() => {
      setModalVisible(false);
      // TODO: 调用后端 API 保存
    });
  };

  /* ---------- 表格列定义 ---------- */

  const knowledgeColumns = [
    {
      title: t('knowledge.table.title'),
      dataIndex: 'title',
      key: 'title',
      /** 标题可点击，打开详情 */
      render: (title: string, record: KnowledgeItem) => (
        <a onClick={() => handleViewDetail(record)} style={{ fontWeight: 500 }}>
          {title}
        </a>
      ),
    },
    {
      title: t('knowledge.table.category'),
      dataIndex: 'category',
      key: 'category',
      width: 100,
      render: (category: string) => <Tag>{category}</Tag>,
    },
    {
      title: t('knowledge.table.tags'),
      dataIndex: 'tags',
      key: 'tags',
      width: 200,
      /** 渲染标签列表 */
      render: (tags: string[]) => (
        <Space size={4} wrap>
          {tags.map((tag) => (
            <Tag key={tag} color="blue">{tag}</Tag>
          ))}
        </Space>
      ),
    },
    {
      title: t('knowledge.table.author'),
      dataIndex: 'author',
      key: 'author',
      width: 100,
      render: (author: string) => (
        <Space size={4}>
          <UserOutlined />
          <Text>{author}</Text>
        </Space>
      ),
    },
    {
      title: t('knowledge.table.updatedAt'),
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      width: 160,
      sorter: (a: KnowledgeItem, b: KnowledgeItem) =>
        new Date(a.updatedAt).getTime() - new Date(b.updatedAt).getTime(),
      render: (time: string) => (
        <Space size={4}>
          <ClockCircleOutlined style={{ color: '#86909C' }} />
          <Text type="secondary">{time}</Text>
        </Space>
      ),
    },
    {
      title: t('knowledge.table.views'),
      dataIndex: 'views',
      key: 'views',
      width: 80,
      sorter: (a: KnowledgeItem, b: KnowledgeItem) => a.views - b.views,
      render: (views: number) => (
        <Space size={4}>
          <EyeOutlined style={{ color: '#86909C' }} />
          <Text>{views.toLocaleString()}</Text>
        </Space>
      ),
    },
    {
      title: t('knowledge.table.likes'),
      dataIndex: 'likes',
      key: 'likes',
      width: 80,
      sorter: (a: KnowledgeItem, b: KnowledgeItem) => a.likes - b.likes,
      /** 点赞按钮，已点赞显示填充图标 */
      render: (likes: number, record: KnowledgeItem) => {
        const liked = likedKeys.has(record.key);
        return (
          <Space
            size={4}
            style={{ cursor: 'pointer' }}
            onClick={() => handleLike(record.key)}
          >
            {liked
              ? <LikeFilled style={{ color: '#4da6ff' }} />
              : <LikeOutlined style={{ color: '#86909C' }} />
            }
            <Text>{liked ? likes + 1 : likes}</Text>
          </Space>
        );
      },
    },
    {
      title: t('knowledge.table.actions'),
      key: 'actions',
      width: 100,
      /** 编辑/删除操作 */
      render: (_: unknown, record: KnowledgeItem) => (
        <Space>
          <Tooltip title={t('knowledge.action.edit')}>
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)} />
          </Tooltip>
          <Tooltip title={t('knowledge.action.delete')}>
            <Button type="link" size="small" danger icon={<DeleteOutlined />} />
          </Tooltip>
        </Space>
      ),
    },
  ];

  /* ---------- 渲染 ---------- */

  return (
    <div>
      {/* 页面标题 + 新建按钮 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('knowledge.title')}</Text>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          {t('knowledge.create')}
        </Button>
      </div>

      <Row gutter={16}>
        {/* 左侧：知识分类树 + 热门标签 */}
        <Col flex="240px">
          {/* 分类树 */}
          <Card
            title={
              <Space>
                <FolderOutlined />
                <span>{t('knowledge.categoryTree.title')}</span>
              </Space>
            }
            style={{ borderRadius: 8, marginBottom: 16 }}
            bodyStyle={{ padding: '8px 12px' }}
          >
            <Tree
              showIcon
              defaultExpandAll
              selectedKeys={[selectedCategory]}
              onSelect={handleCategorySelect}
              treeData={CATEGORY_TREE}
            />
          </Card>

          {/* 热门标签云 */}
          <Card
            title={
              <Space>
                <TagsOutlined />
                <span>{t('knowledge.hotTags.title')}</span>
              </Space>
            }
            style={{ borderRadius: 8 }}
            bodyStyle={{ padding: 12 }}
          >
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
              {HOT_TAGS.map((tag) => (
                <Tag
                  key={tag.name}
                  color={tag.color}
                  style={{
                    cursor: 'pointer',
                    fontSize: Math.min(10 + tag.count / 5, 16),
                    padding: '2px 8px',
                  }}
                  onClick={() => setSearchText(tag.name)}
                >
                  {tag.name}
                  <Badge
                    count={tag.count}
                    size="small"
                    style={{ marginLeft: 4, backgroundColor: 'rgba(0,0,0,0.15)' }}
                  />
                </Tag>
              ))}
            </div>
          </Card>
        </Col>

        {/* 右侧：搜索栏 + 知识列表 */}
        <Col flex="1">
          {/* 搜索栏 */}
          <Card style={{ borderRadius: 8, marginBottom: 16 }} bodyStyle={{ padding: 12 }}>
            <Space style={{ width: '100%' }} wrap>
              {/* 关键词搜索 */}
              <Search
                placeholder={t('knowledge.search.placeholder')}
                value={searchText}
                onChange={(e) => setSearchText(e.target.value)}
                onSearch={setSearchText}
                style={{ width: 300 }}
                allowClear
              />
              {/* 分类过滤下拉 */}
              <Select
                placeholder={t('knowledge.search.categoryFilter')}
                style={{ width: 160 }}
                value={categoryFilter}
                onChange={(val) => {
                  setCategoryFilter(val);
                  setSelectedCategory('all');
                }}
                allowClear
                options={[
                  { value: 'fault-handling', label: '故障处理' },
                  { value: 'ops-manual', label: '运维手册' },
                  { value: 'best-practice', label: '最佳实践' },
                  { value: 'faq', label: 'FAQ' },
                ]}
              />
              {/* 结果计数 */}
              <Text type="secondary">
                {t('knowledge.search.resultCount', { count: filteredKnowledge.length })}
              </Text>
            </Space>
          </Card>

          {/* 知识列表表格 */}
          <Card style={{ borderRadius: 8 }}>
            <Table<KnowledgeItem>
              columns={knowledgeColumns}
              dataSource={filteredKnowledge}
              pagination={{ pageSize: 8, showSizeChanger: true }}
              size="middle"
            />
          </Card>
        </Col>
      </Row>

      {/* 知识详情 Drawer */}
      <Drawer
        title={currentItem?.title}
        open={drawerVisible}
        onClose={() => setDrawerVisible(false)}
        width={640}
        extra={
          <Space>
            <Button icon={<EditOutlined />} onClick={() => currentItem && handleEdit(currentItem)}>
              {t('knowledge.detail.edit')}
            </Button>
          </Space>
        }
      >
        {currentItem && (
          <div>
            {/* 元信息 */}
            <div style={{ marginBottom: 16, display: 'flex', gap: 16, flexWrap: 'wrap' }}>
              <Tag color="blue">{currentItem.category}</Tag>
              <Space size={4}>
                <UserOutlined />
                <Text type="secondary">{currentItem.author}</Text>
              </Space>
              <Space size={4}>
                <ClockCircleOutlined />
                <Text type="secondary">{currentItem.updatedAt}</Text>
              </Space>
              <Space size={4}>
                <EyeOutlined />
                <Text type="secondary">{currentItem.views.toLocaleString()}</Text>
              </Space>
              <Space size={4}>
                <LikeOutlined />
                <Text type="secondary">{currentItem.likes}</Text>
              </Space>
            </div>

            {/* 标签 */}
            <div style={{ marginBottom: 16 }}>
              {currentItem.tags.map((tag) => (
                <Tag key={tag} color="blue" style={{ marginBottom: 4 }}>{tag}</Tag>
              ))}
            </div>

            {/* Markdown 内容渲染（简化版：使用 pre 标签渲染） */}
            <div
              style={{
                padding: 16,
                backgroundColor: '#fafafa',
                borderRadius: 8,
                fontSize: 14,
                lineHeight: 1.8,
                whiteSpace: 'pre-wrap',
                fontFamily: 'inherit',
              }}
            >
              {currentItem.content}
            </div>
          </div>
        )}
      </Drawer>

      {/* 创建/编辑知识弹窗 */}
      <Modal
        title={isEditing ? t('knowledge.modal.editTitle') : t('knowledge.modal.createTitle')}
        open={modalVisible}
        onCancel={() => setModalVisible(false)}
        onOk={handleSave}
        width={720}
        okText={t('knowledge.modal.save')}
        cancelText={t('knowledge.modal.cancel')}
      >
        <Form form={form} layout="vertical">
          {/* 标题 */}
          <Form.Item
            name="title"
            label={t('knowledge.modal.titleLabel')}
            rules={[{ required: true, message: t('knowledge.modal.titleRequired') }]}
          >
            <Input placeholder={t('knowledge.modal.titlePlaceholder')} />
          </Form.Item>

          {/* 分类 */}
          <Form.Item
            name="category"
            label={t('knowledge.modal.categoryLabel')}
            rules={[{ required: true, message: t('knowledge.modal.categoryRequired') }]}
          >
            <Select
              placeholder={t('knowledge.modal.categoryPlaceholder')}
              options={[
                { value: 'fault-db', label: '故障处理 > 数据库故障' },
                { value: 'fault-network', label: '故障处理 > 网络故障' },
                { value: 'fault-app', label: '故障处理 > 应用故障' },
                { value: 'manual-deploy', label: '运维手册 > 部署手册' },
                { value: 'manual-monitor', label: '运维手册 > 监控配置' },
                { value: 'manual-backup', label: '运维手册 > 备份恢复' },
                { value: 'bp-ha', label: '最佳实践 > 高可用架构' },
                { value: 'bp-perf', label: '最佳实践 > 性能优化' },
                { value: 'faq-common', label: 'FAQ > 常见问题' },
                { value: 'faq-tools', label: 'FAQ > 工具使用' },
              ]}
            />
          </Form.Item>

          {/* 标签 */}
          <Form.Item
            name="tags"
            label={t('knowledge.modal.tagsLabel')}
          >
            <Input placeholder={t('knowledge.modal.tagsPlaceholder')} />
          </Form.Item>

          {/* 内容编辑器 */}
          <Form.Item
            name="content"
            label={t('knowledge.modal.contentLabel')}
            rules={[{ required: true, message: t('knowledge.modal.contentRequired') }]}
          >
            <TextArea
              rows={12}
              placeholder={t('knowledge.modal.contentPlaceholder')}
              style={{ fontFamily: 'monospace' }}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default KnowledgeBase;
