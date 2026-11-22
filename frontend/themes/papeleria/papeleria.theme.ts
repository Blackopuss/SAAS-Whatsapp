// The papeleria theme. `astryx experimental theme editor` wrote this file once and never changes it again.
// The checked settings live in papeleria.settings.json and the generated papeleria.settings.ts;
// change them with `astryx experimental theme editor set papeleria ...`. Add your own changes below: they win over the settings.
import {defineTheme} from '@astryxdesign/core/theme';
import {papeleriaSettings} from './papeleria.settings';

export const papeleriaTheme = defineTheme({
  extends: papeleriaSettings,
  name: 'papeleria',
});
