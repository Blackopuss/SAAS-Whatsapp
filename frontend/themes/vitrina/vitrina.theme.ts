// The vitrina theme. `astryx experimental theme editor` wrote this file once and never changes it again.
// The checked settings live in vitrina.settings.json and the generated vitrina.settings.ts;
// change them with `astryx experimental theme editor set vitrina ...`. Add your own changes below: they win over the settings.
import {defineTheme} from '@astryxdesign/core/theme';
import {vitrinaSettings} from './vitrina.settings';

export const vitrinaTheme = defineTheme({
  extends: vitrinaSettings,
  name: 'vitrina',
});
