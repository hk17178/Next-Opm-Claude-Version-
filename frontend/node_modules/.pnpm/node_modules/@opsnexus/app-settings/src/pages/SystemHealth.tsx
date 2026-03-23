/**
 * 系统健康仪表盘组件（FR-23-004）
 * 展示微服务状态、基础设施连通性、系统资源使用率
 * 自动每 30 秒刷新数据
 */
import React, { useState, useEffect, useCallback } from 'react';
import { Card, Row, Col, Badge, Typography, Progress, Skeleton, Space, Tag, message } from 'antd';
import {
  CloudServerOutlined, DatabaseOutlined, HddOutlined,
  ReloadOutlined, CheckCircleOutlined, WarningOutlined, CloseCircleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { request } from '../api/request';

const { Text, Title } = Typography;

// ==================== 类型定义 ====================

/** 服务健康状态：健康 / 降级 / 不可用 */
export type HealthStatus = 'healthy' | 'degraded' | 'unhealthy';

/** 微服务状态信息 */
export interface ServiceHealth {
  /** 服务名称 */
  name: string;
  /** 服务显示名称 */
  displayName: string;
  /** 健康状态 */
  status: HealthStatus;
  /** 响应延迟（毫秒） */
  latencyMs: number;
  /** 版本号 */
  version: string;
  /** 最近一次健康检查时间 */
  lastCheck: string;
}

/** 基础设施组件状态 */
export interface InfraHealth {
  /** 组件名称 */
  name: string;
  /** 组件显示名称 */
  displayName: string;
  /** 连通状态 */
  status: HealthStatus;
  /** 响应延迟（毫秒） */
  latencyMs: number;
}

/** 系统资源使用率 */
export interface ResourceUsage {
  /** CPU 使用率（百分比） */
  cpuPercent: number;
  /** 内存使用率（百分比） */
  memoryPercent: number;
  /** 磁盘使用率（百分比） */
  diskPercent: number;
}

/** 系统健康总览数据 */
export interface SystemHealthData {
  /** 微服务状态列表 */
  services: ServiceHealth[];
  /** 基础设施组件状态列表 */
  infrastructure: InfraHealth[];
  /** 系统资源使用率 */
  resources: ResourceUsage;
}

// ==================== 辅助函数 ====================

/** 将健康状态映射为 Badge 状态 */
const statusToBadge = (status: HealthStatus): 'success' | 'warning' | 'error' => {
  const map: Record<HealthStatus, 'success' | 'warning' | 'error'> = {
    healthy: 'success',
    degraded: 'warning',
    unhealthy: 'error',
  };
  return map[status];
};

/** 将健康状态映射为图标 */
const statusIcon = (status: HealthStatus) => {
  if (status === 'healthy') return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
  if (status === 'degraded') return <WarningOutlined style={{ color: '#faad14' }} />;
  return <CloseCircleOutlined style={{ color: '#ff4d4f' }} />;
};

/** 根据使用率百分比返回 Progress 颜色 */
const getProgressColor = (percent: number): string => {
  if (percent < 60) return '#52c41a';   // 绿色：正常
  if (percent < 80) return '#faad14';   // 黄色：警告
  return '#ff4d4f';                      // 红色：危险
};

/** 自动刷新间隔（毫秒） */
const REFRESH_INTERVAL = 30000;

// ==================== 组件 ====================

/**
 * 系统健康仪表盘
 * - 7 个微服务状态卡片（绿/黄/红）
 * - 基础设施连通性面板（Kafka/ES/PostgreSQL/Redis/ClickHouse）
 * - 系统资源使用率（CPU/内存/磁盘）
 * - 每 30 秒自动刷新
 */
const SystemHealth: React.FC = () => {
  const { t } = useTranslation('settings');

  /** 健康数据 */
  const [data, setData] = useState<SystemHealthData | null>(null);
  /** 加载状态 */
  const [loading, setLoading] = useState(true);
  /** 最后刷新时间 */
  const [lastRefresh, setLastRefresh] = useState<Date>(new Date());

  /**
   * 获取系统健康数据
   * 调用统一健康检查 API 端点
   */
  const fetchHealth = useCallback(async () => {
    try {
      const result = await request<SystemHealthData>('/v1/system/health');
      setData(result);
      setLastRefresh(new Date());
    } catch (err) {
      message.error(t('health.loadError'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  /** 页面初始化加载 + 自动定时刷新 */
  useEffect(() => {
    fetchHealth();
    const timer = setInterval(fetchHealth, REFRESH_INTERVAL);
    return () => clearInterval(timer);
  }, [fetchHealth]);

  /** 加载中显示骨架屏 */
  if (loading) {
    return (
      <div>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
          <Text strong style={{ fontSize: 20 }}>{t('health.title')}</Text>
        </div>
        <Skeleton active paragraph={{ rows: 8 }} />
      </div>
    );
  }

  return (
    <div>
      {/* 页面标题与刷新信息 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 20 }}>{t('health.title')}</Text>
        <Space>
          <Text type="secondary" style={{ fontSize: 12 }}>
            <ReloadOutlined /> {t('health.autoRefresh', { seconds: 30 })}
          </Text>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {t('health.lastRefresh')}: {lastRefresh.toLocaleTimeString()}
          </Text>
        </Space>
      </div>

      {/* 微服务状态 */}
      <Card
        title={<Space><CloudServerOutlined />{t('health.services')}</Space>}
        style={{ borderRadius: 8, marginBottom: 16 }}
      >
        <Row gutter={[16, 16]}>
          {data?.services.map((svc) => (
            <Col xs={24} sm={12} md={8} lg={6} key={svc.name}>
              <Card
                size="small"
                style={{ borderRadius: 8 }}
                bodyStyle={{ padding: 16 }}
              >
                {/* 服务名称与状态图标 */}
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                  <Text strong>{svc.displayName}</Text>
                  {statusIcon(svc.status)}
                </div>
                {/* 状态标签 */}
                <Badge status={statusToBadge(svc.status)} text={t(`health.status.${svc.status}`)} />
                {/* 延迟与版本信息 */}
                <div style={{ marginTop: 8, fontSize: 12, color: '#86909C' }}>
                  <div>{t('health.latency')}: {svc.latencyMs}ms</div>
                  <div>{t('health.version')}: {svc.version}</div>
                </div>
              </Card>
            </Col>
          ))}
        </Row>
      </Card>

      {/* 基础设施连通性 */}
      <Card
        title={<Space><DatabaseOutlined />{t('health.infrastructure')}</Space>}
        style={{ borderRadius: 8, marginBottom: 16 }}
      >
        <Row gutter={[16, 16]}>
          {data?.infrastructure.map((infra) => (
            <Col xs={24} sm={12} md={8} lg={4} key={infra.name}>
              <Card
                size="small"
                style={{ borderRadius: 8, textAlign: 'center' }}
                bodyStyle={{ padding: 16 }}
              >
                {/* 组件名称 */}
                <Text strong style={{ display: 'block', marginBottom: 8 }}>{infra.displayName}</Text>
                {/* 状态标签 */}
                <Tag
                  color={infra.status === 'healthy' ? 'success' : infra.status === 'degraded' ? 'warning' : 'error'}
                >
                  {t(`health.status.${infra.status}`)}
                </Tag>
                {/* 延迟信息 */}
                <div style={{ marginTop: 8, fontSize: 12, color: '#86909C' }}>
                  {infra.latencyMs}ms
                </div>
              </Card>
            </Col>
          ))}
        </Row>
      </Card>

      {/* 系统资源使用率 */}
      <Card
        title={<Space><HddOutlined />{t('health.resources')}</Space>}
        style={{ borderRadius: 8 }}
      >
        {data?.resources && (
          <Row gutter={[48, 24]}>
            {/* CPU 使用率 */}
            <Col xs={24} sm={8}>
              <div style={{ textAlign: 'center' }}>
                <Title level={5}>{t('health.cpu')}</Title>
                <Progress
                  type="dashboard"
                  percent={data.resources.cpuPercent}
                  strokeColor={getProgressColor(data.resources.cpuPercent)}
                  format={(percent) => `${percent}%`}
                />
              </div>
            </Col>
            {/* 内存使用率 */}
            <Col xs={24} sm={8}>
              <div style={{ textAlign: 'center' }}>
                <Title level={5}>{t('health.memory')}</Title>
                <Progress
                  type="dashboard"
                  percent={data.resources.memoryPercent}
                  strokeColor={getProgressColor(data.resources.memoryPercent)}
                  format={(percent) => `${percent}%`}
                />
              </div>
            </Col>
            {/* 磁盘使用率 */}
            <Col xs={24} sm={8}>
              <div style={{ textAlign: 'center' }}>
                <Title level={5}>{t('health.disk')}</Title>
                <Progress
                  type="dashboard"
                  percent={data.resources.diskPercent}
                  strokeColor={getProgressColor(data.resources.diskPercent)}
                  format={(percent) => `${percent}%`}
                />
              </div>
            </Col>
          </Row>
        )}
      </Card>
    </div>
  );
};

export default SystemHealth;
