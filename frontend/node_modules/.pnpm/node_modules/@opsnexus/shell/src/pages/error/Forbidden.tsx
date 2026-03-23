/**
 * Forbidden - 403 无权限错误页
 *
 * 用途：当用户无权访问某个页面或资源时展示
 * 包含：友好插图、中文说明、联系管理员提示、返回按钮
 */
import React from 'react';
import { Button, Result, Space } from 'antd';
import { useNavigate } from 'react-router-dom';

const Forbidden: React.FC = () => {
  const navigate = useNavigate();

  return (
    <div style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      minHeight: 'calc(100vh - 120px)',
    }}>
      <Result
        status="403"
        title="403"
        subTitle="抱歉，您没有权限访问此页面。请联系系统管理员获取相应权限。"
        extra={
          <Space>
            <Button onClick={() => navigate(-1)}>返回上一页</Button>
            <Button type="primary" onClick={() => navigate('/')}>
              返回首页
            </Button>
          </Space>
        }
      />
    </div>
  );
};

export default Forbidden;
