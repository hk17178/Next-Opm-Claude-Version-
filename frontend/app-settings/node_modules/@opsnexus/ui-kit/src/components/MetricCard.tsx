import React from 'react';
import { Card, Statistic } from 'antd';
import { ArrowUpOutlined, ArrowDownOutlined } from '@ant-design/icons';

interface MetricCardProps {
  label: string;
  value: number | string;
  trend?: number;
  trendDirection?: 'up' | 'down';
  suffix?: string;
}

export const MetricCard: React.FC<MetricCardProps> = ({
  label,
  value,
  trend,
  trendDirection,
  suffix,
}) => {
  const trendColor = trendDirection === 'up' ? '#F53F3F' : '#00B42A';
  const TrendIcon = trendDirection === 'up' ? ArrowUpOutlined : ArrowDownOutlined;

  return (
    <Card
      bordered
      style={{
        borderRadius: 8,
        boxShadow: '0 2px 8px rgba(0,0,0,0.06)',
      }}
      bodyStyle={{ padding: '16px 20px' }}
    >
      <Statistic
        title={label}
        value={value}
        suffix={suffix}
      />
      {trend !== undefined && (
        <span style={{ color: trendColor, fontSize: 12 }}>
          <TrendIcon /> {Math.abs(trend)} {suffix}
        </span>
      )}
    </Card>
  );
};
