import React from 'react';

interface EmptyStateProps {
  icon?: string;
  text: string;
  subtext?: string;
}

export default function EmptyState({ icon = '📭', text, subtext }: EmptyStateProps) {
  return (
    <div className="glpi-empty">
      <div className="glpi-empty-icon">{icon}</div>
      <div className="glpi-empty-text">{text}</div>
      {subtext && <div className="glpi-empty-sub">{subtext}</div>}
    </div>
  );
}
