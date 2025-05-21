import React from 'react';

interface StatusBadgeProps {
  status: 'active' | 'inactive';
  showText?: boolean;
  className?: string;
}

const StatusBadge: React.FC<StatusBadgeProps> = ({ 
  status, 
  showText = true,
  className = ''
}) => {
  const isActive = status === 'active';
  
  return (
    <div className={`status-cell ${className}`}>
      <div className={`status-tag ${status}`}>
        <div className="status-dot"></div>
        {showText && <span>{isActive ? '활성' : '비활성'}</span>}
      </div>
    </div>
  );
};

export default StatusBadge; 