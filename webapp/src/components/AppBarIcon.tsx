import React from 'react';

const SVG_ICON = (
  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <rect x="3" y="5" width="18" height="14" rx="2" stroke="currentColor" strokeWidth="1.5" fill="none" />
    <path d="M3 10h18" stroke="currentColor" strokeWidth="1.5" />
    <rect x="6" y="12" width="5" height="2" rx="1" fill="currentColor" opacity="0.6" />
    <rect x="6" y="16" width="3" height="2" rx="1" fill="currentColor" opacity="0.4" />
    <rect x="13" y="12" width="5" height="2" rx="1" fill="currentColor" opacity="0.6" />
  </svg>
);

export default function GLPIAppIcon() {
  return (
    <div
      title="GLPI — IT Support"
      style={{
        width: '100%',
        height: '100%',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        color: 'var(--center-channel-color-80, #555)',
      }}
    >
      {SVG_ICON}
    </div>
  );
}
