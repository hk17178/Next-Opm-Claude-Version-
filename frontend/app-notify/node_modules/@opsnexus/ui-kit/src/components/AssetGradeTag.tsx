import React from 'react';
import { Tag } from 'antd';
import type { AssetGrade } from '../types';
import { ASSET_GRADE_COLORS } from '../theme';

interface AssetGradeTagProps {
  grade: AssetGrade;
}

export const AssetGradeTag: React.FC<AssetGradeTagProps> = ({ grade }) => {
  const color = ASSET_GRADE_COLORS[grade];
  return (
    <Tag color={color} style={{ borderRadius: 4, fontWeight: 600 }}>
      {grade}
    </Tag>
  );
};
