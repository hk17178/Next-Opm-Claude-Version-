import type { RegistrableApp } from 'qiankun';

const DEV_PORT_MAP: Record<string, number> = {
  dashboard: 3001,
  cockpit: 3002,
  alert: 3003,
  incident: 3004,
  log: 3005,
  cmdb: 3006,
  notify: 3007,
  analytics: 3008,
  settings: 3009,
};

function getEntry(name: string): string {
  if (import.meta.env.DEV) {
    return `//localhost:${DEV_PORT_MAP[name]}`;
  }
  return `/subapps/${name}/`;
}

export const microApps: RegistrableApp<Record<string, unknown>>[] = [
  {
    name: 'app-dashboard',
    entry: getEntry('dashboard'),
    container: '#micro-app-container',
    activeRule: (location: Location) =>
      location.pathname.startsWith('/dashboard') || location.pathname.startsWith('/home'),
  },
  {
    name: 'app-cockpit',
    entry: getEntry('cockpit'),
    container: '#micro-app-container',
    activeRule: '/cockpit',
  },
  {
    name: 'app-alert',
    entry: getEntry('alert'),
    container: '#micro-app-container',
    activeRule: '/alert',
  },
  {
    name: 'app-incident',
    entry: getEntry('incident'),
    container: '#micro-app-container',
    activeRule: '/incident',
  },
  {
    name: 'app-log',
    entry: getEntry('log'),
    container: '#micro-app-container',
    activeRule: '/log',
  },
  {
    name: 'app-cmdb',
    entry: getEntry('cmdb'),
    container: '#micro-app-container',
    activeRule: '/cmdb',
  },
  {
    name: 'app-notify',
    entry: getEntry('notify'),
    container: '#micro-app-container',
    activeRule: '/notify',
  },
  {
    name: 'app-analytics',
    entry: getEntry('analytics'),
    container: '#micro-app-container',
    activeRule: '/analytics',
  },
  {
    name: 'app-settings',
    entry: getEntry('settings'),
    container: '#micro-app-container',
    activeRule: '/settings',
  },
];
