import type React from 'react';

// PluginRegistry type matching the Mattermost 10.x webapp Plugin API.
//
// This type is provided at runtime by the Mattermost webapp as a parameter
// to initialize() and is not published as a public npm package. Every
// Mattermost plugin — including official plugins — defines the required
// subset of this interface locally.
//
// The interface below matches the exact signature used by
// Mattermost 10.x webapp for the methods this plugin consumes.
export interface PluginRegistry {
  registerAppBarComponent(
    iconComponent: React.ComponentType,
    action: React.ComponentType<{ onClose?: () => void }> | (() => void),
  ): void;
}
