import React from 'react';

interface ServiceCounterProps {
  total: number;
  active: number;
  inactive: number;
}

const ServiceCounter: React.FC<ServiceCounterProps> = ({
  total,
  active,
  inactive
}) => {
  return (
    <div className="service-counter">
      <div className="counter-box counter-blue">
        <svg viewBox="64 64 896 896" focusable="false" data-icon="appstore" width="1em" height="1em" fill="currentColor" aria-hidden="true" className="counter-icon">
          <path d="M464 144H160c-8.8 0-16 7.2-16 16v304c0 8.8 7.2 16 16 16h304c8.8 0 16-7.2 16-16V160c0-8.8-7.2-16-16-16zm-52 268H212V212h200v200zm452-268H560c-8.8 0-16 7.2-16 16v304c0 8.8 7.2 16 16 16h304c8.8 0 16-7.2 16-16V160c0-8.8-7.2-16-16-16zm-52 268H612V212h200v200zM464 544H160c-8.8 0-16 7.2-16 16v304c0 8.8 7.2 16 16 16h304c8.8 0 16-7.2 16-16V560c0-8.8-7.2-16-16-16zm-52 268H212V612h200v200zm452-268H560c-8.8 0-16 7.2-16 16v304c0 8.8 7.2 16 16 16h304c8.8 0 16-7.2 16-16V560c0-8.8-7.2-16-16-16zm-52 268H612V612h200v200z"></path>
        </svg>
        <span className="counter-number">{total}</span>
      </div>
      
      <div className="counter-box counter-green">
        <svg viewBox="0 0 1024 1024" focusable="false" data-icon="caret-right" width="1em" height="1em" fill="currentColor" aria-hidden="true" className="counter-icon">
          <path d="M715.8 493.5L335 165.1c-14.2-12.2-35-1.2-35 18.5v656.8c0 19.7 20.8 30.7 35 18.5l380.8-328.4c10.9-9.4 10.9-27.6 0-37z"></path>
        </svg>
        <span className="counter-number">{active}</span>
      </div>
      
      <div className="counter-box counter-gray">
        <svg viewBox="64 64 896 896" focusable="false" data-icon="pause" width="1em" height="1em" fill="currentColor" aria-hidden="true" className="counter-icon">
          <path d="M304 176h80v672h-80zm408 0h-64c-4.4 0-8 3.6-8 8v656c0 4.4 3.6 8 8 8h64c4.4 0 8-3.6 8-8V184c0-4.4-3.6-8-8-8z"></path>
        </svg>
        <span className="counter-number">{inactive}</span>
      </div>
    </div>
  );
};

export default ServiceCounter; 