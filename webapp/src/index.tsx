import React from 'react';
import type { PluginRegistry } from './plugin';

import GLPISidebar from './components/GLPISidebar';

declare global {
  interface Window {
    registerPlugin(pluginId: string, plugin: unknown): void;
  }
}

// SVG AppBar icon encoded as a data URI.
// registerAppBarComponent requires a URL string (rendered as <img src={iconUrl}>),
// NOT a React component. This data URI preserves the visual design of the
// existing GLPIAppIcon component for use in the Mattermost AppBar.
function svgDataUri(svg: string): string {
  return 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(svg);
}

const APP_ICON_URL = svgDataUri(
  '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">' +
  '<rect x="3" y="5" width="18" height="14" rx="2" stroke="#555" stroke-width="1.5" fill="none"/>' +
  '<path d="M3 10h18" stroke="#555" stroke-width="1.5"/>' +
  '<rect x="6" y="12" width="5" height="2" rx="1" fill="#555" opacity="0.6"/>' +
  '<rect x="6" y="16" width="3" height="2" rx="1" fill="#555" opacity="0.4"/>' +
  '<rect x="13" y="12" width="5" height="2" rx="1" fill="#555" opacity="0.6"/>' +
  '</svg>',
);

class GLPIWebappPlugin {
  private registry: PluginRegistry | null = null;

  initialize(registry: PluginRegistry): void {
    this.registry = registry;

    // Register the AppBar icon and right-hand sidebar in a single call.
    //
    // Official signature (Mattermost 10.x, verified from registry.ts):
    //   registerAppBarComponent(
    //     iconUrl,            // string — image URL rendered as <img src={iconUrl}>
    //     action,             // undefined -> auto-wired to toggle the RHS
    //     tooltipText,        // shown on hover
    //     supportedProductIds,// null = all products
    //     rhsComponent,       // React component rendered in the RHS panel
    //     rhsTitle,           // RHS panel header title
    //   )
    registry.registerAppBarComponent(
      APP_ICON_URL,
      undefined,
      'GLPI — IT Support',
      null,
      GLPISidebar,
      'GLPI',
    );
  }

  uninitialize(): void {
    this.registry = null;
  }
}

// Bootstrap the plugin with the Mattermost webapp.
//
// window.registerPlugin is defined by the Mattermost webapp at runtime.
// Each webapp plugin MUST call it at module evaluation time to register
// itself. The webapp's plugin service invokes initialize(registry) once
// the Redux store is ready. Without this call, the plugin class compiles
// into the bundle but Mattermost has no reference to it.
if (typeof window.registerPlugin === 'function') {
  window.registerPlugin('com.ntas.glpi', new GLPIWebappPlugin());
} else {
  console.error(
    'GLPI Plugin: window.registerPlugin is not available. ' +
    'Ensure the Mattermost webapp has loaded before this bundle.',
  );
}
