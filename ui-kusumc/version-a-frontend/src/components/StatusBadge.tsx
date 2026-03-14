type StatusBadgeProps = {
  status: string | null | undefined;
  label?: string;
};

const STATUS_STYLES: Record<string, string> = {
  online: 'bg-emerald-100 text-emerald-800 border border-emerald-200',
  offline: 'bg-red-100 text-red-800 border border-red-200',
  unknown: 'bg-slate-100 text-slate-700 border border-slate-200',
  pending: 'bg-amber-100 text-amber-800 border border-amber-200',
  in_progress: 'bg-sky-100 text-sky-800 border border-sky-200',
  applied: 'bg-emerald-100 text-emerald-800 border border-emerald-200',
  failed: 'bg-red-100 text-red-800 border border-red-200',
  active: 'bg-emerald-100 text-emerald-800 border border-emerald-200',
  inactive: 'bg-slate-200 text-slate-700 border border-slate-300',
  decommissioned: 'bg-amber-100 text-amber-800 border border-amber-200',
  owner: 'bg-indigo-100 text-indigo-800 border border-indigo-200',
  secondary: 'bg-slate-100 text-slate-700 border border-slate-200',
  removed: 'bg-slate-200 text-slate-700 border border-slate-300',
  disabled: 'bg-slate-200 text-slate-700 border border-slate-300',
};

export function StatusBadge({ status, label }: StatusBadgeProps) {
  const normalized = (status ?? 'unknown').toLowerCase();
  const style = STATUS_STYLES[normalized] ?? STATUS_STYLES.unknown;
  const textLabel = label ?? normalized;

  return (
    <span
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold capitalize ${style}`}
    >
      {textLabel}
    </span>
  );
}
