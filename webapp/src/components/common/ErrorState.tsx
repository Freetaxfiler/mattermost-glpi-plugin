import React from 'react';

interface ErrorStateProps {
  message: string;
  onRetry?: () => void;
}

export default function ErrorState({ message, onRetry }: ErrorStateProps) {
  return (
    <div className="glpi-error">
      <div className="glpi-error-icon">⚠️</div>
      <div className="glpi-error-text">{message}</div>
      {onRetry && (
        <button className="glpi-btn glpi-btn-secondary glpi-btn-sm" onClick={onRetry}>
          Try again
        </button>
      )}
    </div>
  );
}
