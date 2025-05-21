import React from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import Services from './pages/Services';
import InfraManage from './pages/InfraManage';
import InfraSettings from './pages/InfraSettings';
import Layout from './components/Layout';
import { UserInfo, UserGroupInfo } from './types/user';

function App() {
  // 임시 사용자 정보와 그룹 정보
  const mockUserInfo: UserInfo = {
    id: 1,
    username: 'admin',
    email: 'admin@example.com',
    role: 'admin'
  };

  const mockGroupInfo: UserGroupInfo = {
    id: 1,
    name: '기본 그룹',
    description: '기본 사용자 그룹'
  };

  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route index element={<Navigate to="/services" replace />} />
        <Route path="/services" element={<Services />} />
        <Route path="/infrastructure" element={<InfraManage userInfo={mockUserInfo} groupInfo={mockGroupInfo} />} />
        <Route path="/settings" element={<InfraSettings userInfo={mockUserInfo} groupInfo={mockGroupInfo} />} />
        <Route path="/settings/:infraId" element={<InfraSettings userInfo={mockUserInfo} groupInfo={mockGroupInfo} />} />
      </Route>
    </Routes>
  );
}

export default App;