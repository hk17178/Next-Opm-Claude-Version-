const path = require('path');

module.exports = {
  input: [
    'app-*/src/**/*.{ts,tsx}',
    'shell/src/**/*.{ts,tsx}',
    '!**/node_modules/**',
    '!**/*.test.{ts,tsx}',
    '!**/*.spec.{ts,tsx}',
  ],
  output: './',
  options: {
    debug: false,
    removeUnusedKeys: false,
    sort: true,
    func: {
      list: ['t', 'i18next.t', 'i18n.t'],
      extensions: ['.ts', '.tsx'],
    },
    trans: {
      component: 'Trans',
      i18nKey: 'i18nKey',
      defaultsKey: 'defaults',
      extensions: ['.tsx'],
    },
    lngs: ['zh', 'en'],
    ns: [
      'common',
      'log',
      'alert',
      'incident',
      'cockpit',
      'cmdb',
      'notify',
      'analytics',
      'settings',
      'dashboard',
    ],
    defaultLng: 'zh',
    defaultNs: 'common',
    defaultValue: '__NOT_TRANSLATED__',
    resource: {
      loadPath: '{{ns}}/src/locales/{{lng}}/{{ns}}.json',
      savePath: '{{ns}}/src/locales/{{lng}}/{{ns}}.json',
      jsonIndent: 2,
      lineEnding: '\n',
    },
    nsSeparator: ':',
    keySeparator: '.',
    interpolation: {
      prefix: '{{',
      suffix: '}}',
    },
  },
  transform: function customTransform(file, enc, done) {
    const parser = this.parser;
    const content = file.contents.toString(enc);

    // Extract namespace from file path
    // e.g., app-alert/src/... -> alert namespace
    const relativePath = path.relative(process.cwd(), file.path);
    const appMatch = relativePath.match(/^app-(\w+)\//);
    const isShell = relativePath.startsWith('shell/');

    let defaultNs = 'common';
    if (appMatch) {
      defaultNs = appMatch[1];
    } else if (isShell) {
      defaultNs = 'common';
    }

    // Parse t() calls
    parser.parseFuncFromString(content, {
      list: ['t', 'i18next.t', 'i18n.t'],
    });

    done();
  },
};
