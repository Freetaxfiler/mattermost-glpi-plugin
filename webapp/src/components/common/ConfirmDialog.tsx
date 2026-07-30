import React from 'react';

interface ConfirmDialogProps {
  title: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  onConfirm: () => void;
  onCancel: () => void;
  variant?: 'danger' | 'primary';
}

export default function ConfirmDialog({
  title,
  message,
  confirmText = 'Confirm',
  cancelText = 'Cancel',
  onConfirm,
  onCancel,
  variant = 'primary',
}: ConfirmDialogProps) {
  return (
    <div className="glpi-overlay" onClick={onCancel}>
      <div className="glpi-dialog" onClick={(e) => e.stopPropagation()}>
        <h3>{title}</h3>
        <p>{message}</p>
        <div className="glpi-dialog-actions">
          <button className="glpi-btn glpi-btn-secondary glpi-btn-sm" onClick={onCancel}>
            {cancelText}
          </button>
          <button
            className={`glpi-btn glpi-btn-sm ${variant === 'danger' ? 'glpi-btn-danger' : 'glpi-btn-primary'}`}
            onClick={onConfirm}
          >
            {confirmText}
          </button>
        </div>
      </div>
    </div>
  );
}
