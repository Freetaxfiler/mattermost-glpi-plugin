import React from 'react';

const STATUS_LABELS: Record<number, string> = {
  1: 'New',
  2: 'Processing',
  3: 'Planned',
  4: 'Pending',
  5: 'Solved',
  6: 'Closed',
};

const STATUS_CLASSES: Record<number, string> = {
  1: 'glpi-badge-new',
  2: 'glpi-badge-processing',
  3: 'glpi-badge-planned',
  4: 'glpi-badge-pending',
  5: 'glpi-badge-solved',
  6: 'glpi-badge-closed',
};

export function StatusBadge({ status }: { status: number }) {
  const label = STATUS_LABELS[status] || `Unknown (${status})`;
  const cls = STATUS_CLASSES[status] || '';
  return <span className={`glpi-badge ${cls}`}>{label}</span>;
}

const PRIORITY_CLASSES: Record<number, string> = {
  1: 'glpi-badge-p1',
  2: 'glpi-badge-p2',
  3: 'glpi-badge-p3',
  4: 'glpi-badge-p4',
  5: 'glpi-badge-p5',
};

export function PriorityDot({ priority }: { priority: number }) {
  const cls = PRIORITY_CLASSES[priority] || 'glpi-badge-p3';
  return <span className={`glpi-badge-priority ${cls}`} />;
}
