import React from 'react';
import { Tag } from 'antd';
import type { RootCauseCategory } from '../types';
import { ROOT_CAUSE_COLORS } from '../theme';

interface RootCauseBadgeProps {
  category: RootCauseCategory;
  label: string;
}

export const RootCauseBadge: React.FC<RootCauseBadgeProps> = ({ category, label }) => {
  const color = ROOT_CAUSE_COLORS[category];
  return <Tag color={color}>{label}</Tag>;
};
