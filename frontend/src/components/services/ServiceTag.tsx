import React from 'react';
import { Typography, Space } from 'antd';
import { AppstoreOutlined } from '@ant-design/icons';

interface ServiceTagProps {}

const ServiceTag: React.FC<ServiceTagProps> = () => {
  return (
    <div className="service-title">
      <Space align="center" size={12}>
        <AppstoreOutlined style={{ fontSize: '24px', color: '#1890ff' }} />
        <Typography.Title level={4} style={{ margin: 0, fontWeight: 600 }}>서비스 운영 관리</Typography.Title>
      </Space>
    </div>
  );
};

export default ServiceTag; 