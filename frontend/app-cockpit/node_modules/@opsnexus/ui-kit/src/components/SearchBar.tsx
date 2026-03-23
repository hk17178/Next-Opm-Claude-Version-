import React from 'react';
import { Input, AutoComplete } from 'antd';
import { SearchOutlined } from '@ant-design/icons';

interface Field {
  label: string;
  value: string;
}

interface SearchBarProps {
  fields?: Field[];
  placeholder?: string;
  onSearch: (value: string) => void;
}

export const SearchBar: React.FC<SearchBarProps> = ({ fields, placeholder, onSearch }) => {
  const options = fields?.map((f) => ({
    value: `${f.value}:`,
    label: `${f.label} (${f.value})`,
  }));

  return (
    <AutoComplete
      options={options}
      style={{ width: '100%' }}
      filterOption={(input, option) =>
        (option?.value ?? '').toLowerCase().includes(input.toLowerCase())
      }
    >
      <Input.Search
        prefix={<SearchOutlined />}
        placeholder={placeholder}
        onSearch={onSearch}
        enterButton
        allowClear
        style={{ borderRadius: 6 }}
      />
    </AutoComplete>
  );
};
