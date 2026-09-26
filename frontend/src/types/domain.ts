
export interface DomainRecord {
  id: number;
  code: string;
  name: string;
  status: string;
  version: number;
  description: string;
  facility: string;
  owner: string;
  category: string;
  riskLevel: 'low' | 'medium' | 'high' | 'critical';
  metricValue: number;
  metricUnit: string;
  effectiveAt: string;
  evidence: string;
  relatedCode: string;
  preparedBy?: string;
  revisions?: PriorityDecisionRevision[];
  /** Derived by the backend: finalized restrict/urgent decision can be released once all same-bridge defects are mitigated/closed. */
  releaseEligible?: boolean;
  /** Defect codes on the same bridge that are not yet mitigated/closed. */
  outstandingDefectCodes?: string[];
  createdAt: string;
  updatedAt: string;
}

export interface PriorityDecisionRevision {
  id: number;
  priorityDecisionId: number;
  version: number;
  status: string;
  evidence: string;
  reason: string;
  actor: string;
  requestId: string;
  snapshot: string;
  createdAt: string;
}

export interface PageMeta { page: number; pageSize: number; total: number }
export interface ApiEnvelope<T> { data: T; error?: string; message?: string; meta?: PageMeta }
export type UserRole = 'viewer' | 'operator' | 'reviewer' | 'admin';
export interface UserSession { token: string; username: string; displayName: string; role: UserRole; expiresIn: number }
export interface AuditLog {
  id: number; requestId: string; actor: string; action: string; entityType: string;
  entityId: number; beforeState: string; afterState: string; detail: string; createdAt: string;
}
export interface EntityConfig { key: string; path: string; label: string; statuses: readonly string[] }
