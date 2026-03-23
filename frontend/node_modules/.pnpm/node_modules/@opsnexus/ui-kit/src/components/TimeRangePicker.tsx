import React from 'react';
import { DatePicker, Select, Space } from 'antd';
import type { Dayjs } from 'dayjs';

const { RangePicker } = DatePicker;

interface TimeRangePickerProps {
  presets?: string[];
  onChange: (range: [Dayjs, Dayjs] | null, preset?: string) => void;
}

const DEFAULT_PRESETS = ['15min', '1h', '6h', '24h', '7d', '30d'];

export const TimeRangePicker: React.FC<TimeRangePickerProps> = ({
  presets = DEFAULT_PRESETS,
  onChange,
}) => {
  return (
    <Space>
      <Select
        placeholder="Quick"
        style={{ width: 120 }}
        options={presets.map((p) => ({ value: p, label: p }))}
        onChange={(v) => onChange(null, v)}
        allowClear
      />
      <RangePicker
        showTime
        onChange={(dates) => onChange(dates as [Dayjs, Dayjs])}
      />
    </Space>
  );
};
