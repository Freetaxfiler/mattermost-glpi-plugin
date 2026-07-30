import type { PluginRegistry } from './plugin';

import GLPIAppIcon from './components/AppBarIcon';
import GLPISidebar from './components/GLPISidebar';

export default class GLPIWebAppPlugin {
  private registry: PluginRegistry | null = null;

  initialize(registry: PluginRegistry) {
    this.registry = registry;

    // Register the Apps Bar component — this adds the GLPI icon to the
    // Mattermost Apps Bar (left sidebar). Clicking it opens the right sidebar.
    registry.registerAppBarComponent(
      GLPIAppIcon,
      () => <GLPISidebar onClose={() => this.closeSidebar()} />,
    );
  }

  uninitialize() {
    this.registry = null;
  }

  private closeSidebar() {
    // The sidebar close is handled by the component itself.
    // This method exists for future cleanup if needed.
  }
}
