// The panel theme. `astryx experimental theme editor` wrote this file once and never changes it again.
// The checked settings live in panel.settings.json and the generated panel.settings.ts;
// change them with `astryx experimental theme editor set panel ...`. Add your own changes below: they win over the settings.
import {defineTheme} from '@astryxdesign/core/theme';
import {panelSettings} from './panel.settings';

export const panelTheme = defineTheme({
  extends: panelSettings,
  name: 'panel',
});
