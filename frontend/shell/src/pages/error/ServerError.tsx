/**
 * ServerError - 500 系统异常错误页
 *
 * 用途：当服务端出现未预期的错误时展示
 * 包含：友好插图、中文说明、重试按钮、返回首页按钮
 */
import React from 'react';
import { Button, Result, Space } from 'antd';
import { useNavigate } from 'react-router-dom';

const ServerError: React.FC = () => {
  const navigate = useNavigate();

  return (
    <div style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      minHeight: 'calc(100vh - 120px)',
    }}>
      <Result
        status="500"
        title="500"
        subTitle="抱歉，系统遇到了一些问题。我们的工程师正在紧急修复中。"
        extra={
          <Space>
            <Button type="primary" onClick={() => window.location.reload()}>
              重新加载
            </Button>
            <Button onClick={() => navigate('/')}>
              返回首页
            </Button>
          </Space>
        }
      />
    </div>
  );
};

export default ServerError;
