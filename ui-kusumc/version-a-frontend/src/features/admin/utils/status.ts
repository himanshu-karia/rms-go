import type { AdminProject, AdminVendor } from '../../../api/admin';
import type { UserGroupSummary } from '../../../api/userGroups';

type MetadataShape = Record<string, unknown> | null | undefined;

export type EntityStatus = 'active' | 'inactive';

export function deriveStatus(metadata: MetadataShape): EntityStatus | undefined {
  if (!metadata || typeof metadata !== 'object') {
    return undefined;
  }

  const raw = (metadata as Record<string, unknown>).status;
  if (typeof raw !== 'string') {
    return undefined;
  }

  const normalized = raw.trim().toLowerCase();
  if (normalized === 'active' || normalized === 'inactive') {
    return normalized;
  }

  return undefined;
}

export function isActiveProject(project: AdminProject): boolean {
  return deriveStatus(project.metadata) === 'active';
}

export function isActiveVendor(vendor: AdminVendor): boolean {
  return deriveStatus(vendor.metadata) === 'active';
}

export function isActiveGroup(group: UserGroupSummary): boolean {
  return deriveStatus(group.metadata) === 'active';
}
