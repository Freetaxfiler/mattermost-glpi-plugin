import React from 'react';

interface LoadingProps {
  text?: string;
}

export default function Loading({ text = 'Loading...' }: LoadingProps) {
  return (
    <div className="glpi-loading">
      <div className="glpi-spinner" />
      <span>{text}</span>
    </div>
  );
}
