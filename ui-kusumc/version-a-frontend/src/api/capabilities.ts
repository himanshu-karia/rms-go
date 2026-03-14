export type CapabilityKey =
  | 'telemetry:read'
  | 'telemetry:live:device'
  | 'telemetry:live:all'
  | 'telemetry:export'
  | 'alerts:manage'
  | 'reports:manage'
  | 'devices:read'
  | 'devices:write'
  | 'devices:credentials'
  | 'devices:commands'
  | 'devices:bulk_import'
  | 'simulator:launch'
  | 'simulator:commands'
  | 'diagnostics:read'
  | 'diagnostics:commands'
  | 'catalog:protocols'
  | 'catalog:drives'
  | 'catalog:rs485'
  | 'hierarchy:manage'
  | 'vendors:manage'
  | 'installations:manage'
  | 'beneficiaries:manage'
  | 'users:manage'
  | 'audit:read'
  | 'support:manage'
  | 'knowledge_base:manage'
  | 'admin:all';

export type CapabilityMatchMode = 'all' | 'any';

type CapabilityList = CapabilityKey | CapabilityKey[];

export type CapabilityMatchOptions = {
  match?: CapabilityMatchMode;
};

export function hasCapabilities(
  granted: CapabilityKey[] | null | undefined,
  required: CapabilityList,
  options: CapabilityMatchOptions = {},
): boolean {
  const matchMode: CapabilityMatchMode = options.match ?? 'all';
  const requirement = Array.isArray(required) ? required : [required];
  if (!requirement.length) {
    return true;
  }

  const grantedSet = new Set(granted ?? []);
  if (matchMode === 'any') {
    return requirement.some((capability) => grantedSet.has(capability));
  }

  return requirement.every((capability) => grantedSet.has(capability));
}
