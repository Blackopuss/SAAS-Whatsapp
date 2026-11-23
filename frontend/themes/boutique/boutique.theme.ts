// The boutique theme. `astryx experimental theme editor` wrote this file once and never changes it again.
// The checked settings live in boutique.settings.json and the generated boutique.settings.ts;
// change them with `astryx experimental theme editor set boutique ...`. Add your own changes below: they win over the settings.
import {defineTheme} from '@astryxdesign/core/theme';
import {boutiqueSettings} from './boutique.settings';

export const boutiqueTheme = defineTheme({
  extends: boutiqueSettings,
  name: 'boutique',
});
