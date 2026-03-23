/**
 * Maintenance - 503 系统维护中页面
 *
 * 用途：系统处于计划维护期间时展示
 * 包含：维护图标、中文说明、预计恢复时间、刷新按钮
 */
import React from 'react';
import { Button, Result, Typography } from 'antd';
import { ToolOutlined } from '@ant-design/icons';

const { Text } = Typography;

const Maintenance: React.FC = () => {
  return (
    <div style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      minHeight: 'calc(100vh - 120px)',
    }}>
      <Result
        icon={<ToolOutlined style={{ color: '#FF7D00', fontSize: 72 }} />}
        title="系统维护中"
        subTitle="我们正在进行系统升级维护，请稍后再试。"
        extra={[
          <div key="time" style={{ marginBottom: 24 }}>
            <Text type="secondary" style={{ fontSize: 14 }}>
              预计恢复时间：请关注系统公告
            </Text>
          </div>,
          <Button key="refresh" type="primary" onClick={() => window.location.reload()}>
            刷新页面
          </Button>,
        ]}
      />
    </div>
  );
};

export default Maintenance;
