import type { ReactNode } from 'react';

export type AdminKpiItem = {
  id: string;
  label: string;
  value: number | string | null;
  description?: ReactNode;
  tooltip?: string;
};

export type AdminKpiGridProps = {
  items: AdminKpiItem[];
  isLoading?: boolean;
};

function renderValue(value: AdminKpiItem['value'], isLoading?: boolean) {
  if (isLoading) {
    return '…';
  }
  if (value === null) {
    return '—';
  }
  return typeof value === 'number' ? value.toLocaleString() : value;
}

export function AdminKpiGrid({ items, isLoading }: AdminKpiGridProps) {
  if (!items.length) {
    return null;
  }

  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
      {items.map((item) => (
        <article
          key={item.id}
          className="rounded border border-slate-200 bg-slate-50 px-4 py-3 shadow-sm"
        >
          <header className="flex items-center justify-between">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
              {item.label}
            </p>
            {item.tooltip ? (
              <span className="text-xs text-slate-400" title={item.tooltip}>
                ⓘ
              </span>
            ) : null}
          </header>
          <p className="mt-2 text-2xl font-semibold text-slate-900">
            {renderValue(item.value, isLoading)}
          </p>
          {item.description ? (
            <p className="mt-1 text-xs text-slate-500">{item.description}</p>
          ) : null}
        </article>
      ))}
    </div>
  );
}
