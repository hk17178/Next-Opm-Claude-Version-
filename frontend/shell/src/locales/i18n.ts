import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import zhCommon from './zh/common.json';
import enCommon from './en/common.json';

i18n.use(initReactI18next).init({
  resources: {
    zh: { translation: zhCommon },
    en: { translation: enCommon },
  },
  lng: 'zh',
  fallbackLng: 'en',
  interpolation: {
    escapeValue: false,
  },
});

export default i18n;
