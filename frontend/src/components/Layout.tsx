import React from 'react';
import { Outlet, Link, useLocation } from 'react-router-dom';
import { Layout as AntLayout, Menu } from 'antd';
import {
  AppstoreOutlined,
  CloudServerOutlined,
  SettingOutlined
} from '@ant-design/icons';
import './Layout.css';

const { Header, Content } = AntLayout;

const Layout: React.FC = () => {
  const location = useLocation();
  const currentPath = location.pathname;

  return (
    <AntLayout className="system-container">
      <Header className="system-header">
        <div className="header-left">
          <AppstoreOutlined className="header-icon" />
          <h1>K8S Control</h1>
        </div>
      </Header>
      
      <div className="tab-menu">
        <Link 
          to="/services" 
          className={`tab-item ${currentPath === '/' || currentPath.includes('/services') ? 'active' : ''}`}
        >
          <div className="tab-icon">
            <AppstoreOutlined />
          </div>
          <span>서비스운영 관리</span>
        </Link>
        <Link 
          to="/infrastructure" 
          className={`tab-item ${currentPath.includes('/infrastructure') ? 'active' : ''}`}
        >
          <div className="tab-icon">
            <CloudServerOutlined />
          </div>
          <span>인프라 관리</span>
        </Link>
        <Link 
          to="/settings" 
          className={`tab-item ${currentPath.includes('/settings') ? 'active' : ''}`}
        >
          <div className="tab-icon">
            <SettingOutlined />
          </div>
          <span>인프라 설정</span>
        </Link>
      </div>
      
      <Content className="system-content">
        <Outlet />
      </Content>
    </AntLayout>
  );
};

export default Layout; 