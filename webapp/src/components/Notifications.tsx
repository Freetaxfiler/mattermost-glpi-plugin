import React, { useState } from 'react';
import EmptyState from './common/EmptyState';

export default function Notifications() {
  // Notifications are served via webhook and posted to the configured channel.
  // The sidebar displays recent notifications from an internal event log.
  // For v0.2.0, this is a read-only reference view showing how notifications
  // are routed. Future versions will include a persistent notification store.

  return (
    <div>
      <div className="glpi-section">
        <div className="glpi-section-title">Notifications</div>
        <div className="glpi-card">
          <div style={{ fontSize: 13, lineHeight: 1.6 }}>
            <p>GLPI webhook notifications are posted to the configured notification channel and sent as DMs to the ticket requester when their email matches a Mattermost account.</p>
            <p>To configure notifications:</p>
            <ol style={{ paddingLeft: 20, margin: '8px 0' }}>
              <li>Set the <strong>Notification Channel ID</strong> in plugin settings</li>
              <li>Configure a GLPI webhook (Setup &gt; Webhooks) to POST to:</li>
            </ol>
            <code className="glpi-monospace" style={{ display: 'block', padding: 8, background: 'var(--center-channel-bg-10)', borderRadius: 4, fontSize: 11, wordBreak: 'break-all' }}>
              /plugins/com.ntas.glpi/webhook
            </code>
            <p style={{ marginTop: 10 }}>
              Include the <code>X-GLPI-Secret</code> header with your configured webhook secret.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
