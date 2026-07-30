// Mattermost plugin registry types for the webapp plugin system.
// These describe the subset of the Mattermost WebApp Plugin API used by this plugin.

export interface PluginRegistry {
  registerAppBarComponent(
    iconComponent: React.ComponentType,
    sidebarComponent: React.ComponentType<{ onClose?: () => void }>,
  ): void;
}

// The default export for Mattermost webapp plugins.
// Mattermost loads this as an ESM module and calls the default export
// function to get the plugin instance.
export { default } from './index';
