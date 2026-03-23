export type Severity = 'P0' | 'P1' | 'P2' | 'P3' | 'P4';

export type RootCauseCategory =
  | 'human_action'
  | 'system_fault'
  | 'change_induced'
  | 'external_dependency'
  | 'pending';

export type AssetGrade = 'S' | 'A' | 'B' | 'C' | 'D';

export interface PageParams {
  current: number;
  pageSize: number;
}

export interface ApiResponse<T = unknown> {
  code: number;
  message: string;
  data: T;
}

export interface PageResult<T = unknown> {
  list: T[];
  total: number;
  current: number;
  pageSize: number;
}

export interface AIAnalysisResult {
  rootCause: string;
  category: RootCauseCategory;
  confidence: number;
  model: string;
  evidences: string[];
  suggestions: string[];
  estimatedRecovery: string;
  duration: number;
}

export interface TimelineEntry {
  time: string;
  type: 'system' | 'human' | 'ai' | 'recovery';
  title: string;
  description?: string;
  risk?: boolean;
}
