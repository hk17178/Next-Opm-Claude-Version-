import React from 'react';
import { Tag } from 'antd';

interface StatusTagProps {
  status: string;
  colorMap?: Record<string, string>;
}

const DEFAULT_COLOR_MAP: Record<string, string> = {
  firing: '#F53F3F',
  acknowledged: '#FF7D00',
  resolved: '#00B42A',
  suppressed: '#86909C',
  active: '#F53F3F',
  processing: '#FF7D00',
  pending_review: '#FF7D00',
  closed: '#86909C',
};

export const StatusTag: React.FC<StatusTagProps> = ({ status, colorMap }) => {
  const colors = { ...DEFAULT_COLOR_MAP, ...colorMap };
  const color = colors[status] || '#86909C';
  return <Tag color={color}>{status}</Tag>;
};
