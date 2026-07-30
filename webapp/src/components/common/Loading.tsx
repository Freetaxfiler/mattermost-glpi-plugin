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

export function Skeleton({ lines = 3 }: { lines?: number }) {
  return (
    <div style={{ padding: '10px 0' }}>
      {Array.from({ length: lines }).map((_, i) => (
        <div
          key={i}
          className="glpi-skeleton"
          style={{ width: `${60 + Math.random() * 30}%` }}
        />
      ))}
    </div>
  );
}
