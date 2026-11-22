// The abarrotes theme. `astryx experimental theme editor` wrote this file once and never changes it again.
// The checked settings live in abarrotes.settings.json and the generated abarrotes.settings.ts;
// change them with `astryx experimental theme editor set abarrotes ...`. Add your own changes below: they win over the settings.
import {defineTheme} from '@astryxdesign/core/theme';
import {abarrotesSettings} from './abarrotes.settings';

export const abarrotesTheme = defineTheme({
  extends: abarrotesSettings,
  name: 'abarrotes',
});
