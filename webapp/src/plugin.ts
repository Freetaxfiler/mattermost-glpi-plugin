import type React from 'react';

// PluginRegistry matching Mattermost 10.x webapp Plugin API.
//
// Verified from Mattermost's own registry.ts (release-10.10 and master):
//   registerAppBarComponent(iconUrl, action, tooltipText, supportedProductIds, rhsComponent, rhsTitle)
//
// The first parameter (iconUrl) is a URL STRING pointing to the icon image,
// not a React component. The Mattermost AppBar renders icons via <img src={iconUrl}>.
// When rhsComponent is provided, clicking the icon toggles the right-hand sidebar.
//
// This interface is not published as an npm package; every official Mattermost
// plugin defines the required subset locally.
export interface PluginRegistry {
  registerAppBarComponent(
    iconUrl: string,
    action: undefined,
    tooltipText: React.ReactNode,
    supportedProductIds?: unknown,
    rhsComponent?: React.ComponentType,
    rhsTitle?: React.ReactNode,
  ): void;
  // Registers a handler for a Mattermost WebSocket event. Plugin events are
  // delivered under the server-prefixed name custom_<pluginId>_<event>.
  registerWebSocketEventHandler(
    event: string,
    handler: (msg: { event: string; data?: Record<string, unknown> }) => void,
  ): void;
}
