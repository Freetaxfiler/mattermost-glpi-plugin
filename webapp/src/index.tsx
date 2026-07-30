import React from 'react';
import type { PluginRegistry } from './plugin';

import GLPIAppIcon from './components/AppBarIcon';
import GLPISidebar from './components/GLPISidebar';

declare global {
  interface Window {
    registerPlugin(pluginId: string, plugin: unknown): void;
  }
}

class GLPIWebappPlugin {
  private registry: PluginRegistry | null = null;

  initialize(registry: PluginRegistry): void {
    this.registry = registry;

    registry.registerAppBarComponent(
      GLPIAppIcon,
      GLPISidebar,
    );
  }

  uninitialize(): void {
    this.registry = null;
  }
}

// Bootstrap the plugin with the Mattermost webapp.
//
// Mattermost defines window.registerPlugin on the webapp's global scope.
// Each plugin must call it at module evaluation time to self-register.
// This module-scope side effect anchors the entire plugin into Webpack's
// module graph and prevents tree-shaking from removing any imported
// component or the initialize method. Without this call the plugin class
// compiles into the bundle but Mattermost has no reference to it and no
// App Bar icon appears.
if (typeof window.registerPlugin === 'function') {
  window.registerPlugin('com.ntas.glpi', new GLPIWebappPlugin());
} else {
  console.error(
    'GLPI Plugin: window.registerPlugin is not available. ' +
    'Ensure the Mattermost webapp has loaded before this bundle.',
  );
}
