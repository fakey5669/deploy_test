import React from 'react';
import { Button, Input, Select } from 'antd';
import { PlusOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';

const { Option } = Select;

interface ServiceActionsProps {
  onSearch: (value: string) => void;
  searchTerm: string;
  onRefresh: () => void;
  onCreate: () => void;
}

const ServiceActions: React.FC<ServiceActionsProps> = ({
  onSearch,
  searchTerm,
  onRefresh,
  onCreate
}) => {
  return (
    <div className="service-actions">
      <Button type="primary" icon={<PlusOutlined />} onClick={onCreate}>
        서비스 생성
      </Button>
      <Button icon={<ReloadOutlined />} onClick={onRefresh}>
        새로고침
      </Button>
      <Select defaultValue="모든 상태" style={{ width: 120 }}>
        <Option value="모든 상태">모든 상태</Option>
        <Option value="활성">활성</Option>
        <Option value="비활성">비활성</Option>
      </Select>
      <div className="search-input">
        <Input
          placeholder="서비스 검색"
          value={searchTerm}
          onChange={(e) => onSearch(e.target.value)}
          prefix={<SearchOutlined />}
          allowClear
        />
      </div>
      <Select defaultValue="이름순" style={{ width: 120 }}>
        <Option value="이름순">이름순</Option>
        <Option value="최신순">최신순</Option>
      </Select>
    </div>
  );
};

export default ServiceActions; 