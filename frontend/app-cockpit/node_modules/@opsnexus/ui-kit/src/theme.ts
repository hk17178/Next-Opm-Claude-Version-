import type { Severity, RootCauseCategory, AssetGrade } from './types';

export const SEVERITY_COLORS: Record<Severity, string> = {
  P0: '#F53F3F',
  P1: '#FF7D00',
  P2: '#3491FA',
  P3: '#86909C',
  P4: '#C9CDD4',
};

export const ROOT_CAUSE_COLORS: Record<RootCauseCategory, string> = {
  human_action: '#722ED1',
  system_fault: '#F53F3F',
  change_induced: '#FF7D00',
  external_dependency: '#3491FA',
  pending: '#86909C',
};

export const ASSET_GRADE_COLORS: Record<AssetGrade, string> = {
  S: '#F53F3F',
  A: '#FF7D00',
  B: '#3491FA',
  C: '#86909C',
  D: '#C9CDD4',
};

export const CSS_VARS = {
  light: {
    bgPrimary: '#FFFFFF',
    bgSecondary: '#F7F8FA',
    bgTertiary: '#F0F2F5',
    textPrimary: '#1D2129',
    textSecondary: '#86909C',
    colorPrimary: '#2E75B6',
    colorSuccess: '#00B42A',
    colorWarning: '#FF7D00',
    colorDanger: '#F53F3F',
    colorInfo: '#3491FA',
    borderColor: '#E5E6EB',
  },
  dark: {
    bgPrimary: '#1A1A1A',
    bgSecondary: '#242424',
    bgTertiary: '#141414',
    textPrimary: '#E5E6EB',
    textSecondary: '#A9AEB8',
    colorPrimary: '#4C9AE6',
    colorSuccess: '#27C346',
    colorWarning: '#FF9A2E',
    colorDanger: '#F76560',
    colorInfo: '#5BB7F5',
    borderColor: '#333333',
  },
};
