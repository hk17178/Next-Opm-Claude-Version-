import React from 'react';
import { Select, Button, Space } from 'antd';
import { useTranslation } from 'react-i18next';

interface FilterOption {
  label: string;
  value: string;
}

interface FilterConfig {
  key: string;
  label: string;
  options: FilterOption[];
  multiple?: boolean;
}

interface FilterBarProps {
  filters: FilterConfig[];
  values?: Record<string, string | string[]>;
  onFilter: (values: Record<string, string | string[]>) => void;
  onReset?: () => void;
}

export const FilterBar: React.FC<FilterBarProps> = ({
  filters,
  values = {},
  onFilter,
  onReset,
}) => {
  const { t } = useTranslation();

  const handleChange = (key: string, value: string | string[]) => {
    onFilter({ ...values, [key]: value });
  };

  return (
    <Space wrap>
      {filters.map((f) => (
        <Select
          key={f.key}
          placeholder={f.label}
          style={{ minWidth: 140 }}
          value={values[f.key]}
          options={f.options}
          mode={f.multiple ? 'multiple' : undefined}
          onChange={(v) => handleChange(f.key, v)}
          allowClear
        />
      ))}
      {onReset && (
        <Button onClick={onReset}>{t('common.reset')}</Button>
      )}
    </Space>
  );
};
