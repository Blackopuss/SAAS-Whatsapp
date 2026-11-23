// The barberia theme. `astryx experimental theme editor` wrote this file once and never changes it again.
// The checked settings live in barberia.settings.json and the generated barberia.settings.ts;
// change them with `astryx experimental theme editor set barberia ...`. Add your own changes below: they win over the settings.
import {defineTheme} from '@astryxdesign/core/theme';
import {barberiaSettings} from './barberia.settings';

export const barberiaTheme = defineTheme({
  extends: barberiaSettings,
  name: 'barberia',
});
