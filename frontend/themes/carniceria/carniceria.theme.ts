// The carniceria theme. `astryx experimental theme editor` wrote this file once and never changes it again.
// The checked settings live in carniceria.settings.json and the generated carniceria.settings.ts;
// change them with `astryx experimental theme editor set carniceria ...`. Add your own changes below: they win over the settings.
import {defineTheme} from '@astryxdesign/core/theme';
import {carniceriaSettings} from './carniceria.settings';

export const carniceriaTheme = defineTheme({
  extends: carniceriaSettings,
  name: 'carniceria',
});
