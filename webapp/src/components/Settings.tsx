import React, { useState, useEffect } from 'react';
import { api } from '../api';
import type { StatusResponse, ConfigResponse } from '../types';
import Loading from './common/Loading';

export default function Settings() {
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [config, setConfig] = useState<ConfigResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([api.getStatus(), api.getConfig()])
      .then(([s, c]) => {
        setStatus(s);
        setConfig(c);
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Failed to load settings'))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <Loading text="Loading settings..." />;

  return (
    <div>
      <div className="glpi-section">
        <div className="glpi-section-title">Plugin Settings</div>

        {error && (
          <div className="glpi-card">
            <div className="glpi-form-error">{error}</div>
          </div>
        )}

        {/* Status */}
        <div className="glpi-section-title" style={{ marginTop: 16 }}>Connection Status</div>
        <div className="glpi-card">
          <div className="glpi-flex glpi-flex-between glpi-flex-center">
            <span style={{ fontSize: 13 }}>Status</span>
            <span className={`glpi-indicator ${status?.glpi_online ? 'glpi-indicator-online' : 'glpi-indicator-offline'}`}>
              <span className={`glpi-indicator-dot ${status?.glpi_online ? 'glpi-indicator-dot-on' : 'glpi-indicator-dot-off'}`} />
              {status?.glpi_online ? 'Connected' : 'Disconnected'}
            </span>
          </div>
          {status?.glpi_version && (
            <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
              <span style={{ fontSize: 13 }}>GLPI Version</span>
              <span style={{ fontSize: 13 }}>{status.glpi_version}</span>
            </div>
          )}
          <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
            <span style={{ fontSize: 13 }}>Plugin Version</span>
            <span style={{ fontSize: 13 }}>v{status?.plugin_version || '0.2.0'}</span>
          </div>
        </div>

        {/* Configuration */}
        <div className="glpi-section-title" style={{ marginTop: 16 }}>Configuration</div>
        <div className="glpi-card">
          <div className="glpi-flex glpi-flex-between glpi-flex-center">
            <span style={{ fontSize: 13 }}>GLPI URL</span>
            <span className="glpi-monospace" style={{ fontSize: 12, maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis' }}>
              {config?.glpi_url || 'not set'}
            </span>
          </div>
          <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
            <span style={{ fontSize: 13 }}>Configured</span>
            <span style={{ fontSize: 13, color: status?.configured ? 'var(--online-indicator)' : 'var(--error-text-color)' }}>
              {status?.configured ? 'Yes' : 'No'}
            </span>
          </div>
          <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
            <span style={{ fontSize: 13 }}>Default Entity</span>
            <span style={{ fontSize: 13 }}>{config?.default_entity || '—'}</span>
          </div>
          <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
            <span style={{ fontSize: 13 }}>Default Category</span>
            <span style={{ fontSize: 13 }}>{config?.default_category || '—'}</span>
          </div>
          <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
            <span style={{ fontSize: 13 }}>Notifications</span>
            <span style={{ fontSize: 13 }}>{config?.notification_channel_id ? 'Channel configured' : 'Not configured'}</span>
          </div>
          <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
            <span style={{ fontSize: 13 }}>Debug Logging</span>
            <span style={{ fontSize: 13 }}>{config?.enable_debug ? 'Enabled' : 'Disabled'}</span>
          </div>
        </div>

        {/* Management hint */}
        <div className="glpi-card" style={{ marginTop: 16, background: 'var(--center-channel-bg-5)' }}>
          <div style={{ fontSize: 12, color: 'var(--center-channel-color-60)' }}>
            Configuration is managed in <strong>System Console {'>'} Plugins {'>'} GLPI</strong>.
            Changes made there take effect immediately.
          </div>
        </div>
      </div>
    </div>
  );
}
