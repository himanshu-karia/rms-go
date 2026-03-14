import { ChangeEvent } from 'react';

export type DateRangeValue = {
  from: Date;
  to: Date;
};

export type DateRangePresetConfig = {
  id: string;
  label: string;
  getRange: () => DateRangeValue;
};

type DateRangePickerProps = {
  value: DateRangeValue;
  onChange: (range: DateRangeValue) => void;
  presets?: DateRangePresetConfig[];
  activePresetId?: string | null;
  onPresetSelect?: (presetId: string | null) => void;
  disabled?: boolean;
};

function toInputValue(date: Date) {
  if (Number.isNaN(date.getTime())) {
    return '';
  }
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  return `${year}-${month}-${day}T${hours}:${minutes}`;
}

function parseInputValue(value: string): Date | null {
  if (!value) {
    return null;
  }
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? null : parsed;
}

function normalizeRange(range: DateRangeValue) {
  const { from, to } = range;
  if (from.getTime() > to.getTime()) {
    return { from, to: from } satisfies DateRangeValue;
  }
  return range;
}

export function DateRangePicker({
  value,
  onChange,
  presets,
  activePresetId,
  onPresetSelect,
  disabled,
}: DateRangePickerProps) {
  const handleInputChange = (field: 'from' | 'to') => (event: ChangeEvent<HTMLInputElement>) => {
    if (disabled) {
      return;
    }
    const nextDate = parseInputValue(event.target.value);
    if (!nextDate) {
      return;
    }

    const nextRange = normalizeRange({
      from: field === 'from' ? nextDate : value.from,
      to: field === 'to' ? nextDate : value.to,
    });

    onChange(nextRange);
    onPresetSelect?.(null);
  };

  const handlePresetClick = (preset: DateRangePresetConfig) => {
    if (disabled) {
      return;
    }
    const nextRange = normalizeRange(preset.getRange());
    onChange(nextRange);
    onPresetSelect?.(preset.id);
  };

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-end gap-3">
        <label className="flex flex-col gap-1 text-xs font-medium uppercase tracking-wide text-slate-500">
          <span>From</span>
          <input
            type="datetime-local"
            value={toInputValue(value.from)}
            onChange={handleInputChange('from')}
            className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-800 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
            disabled={disabled}
          />
        </label>
        <label className="flex flex-col gap-1 text-xs font-medium uppercase tracking-wide text-slate-500">
          <span>To</span>
          <input
            type="datetime-local"
            value={toInputValue(value.to)}
            onChange={handleInputChange('to')}
            className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-800 shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500"
            disabled={disabled}
          />
        </label>
      </div>
      {presets && presets.length > 0 && (
        <div className="flex flex-wrap gap-2 text-xs font-semibold">
          {presets.map((preset) => {
            const isActive = preset.id === activePresetId;
            return (
              <button
                key={preset.id}
                type="button"
                onClick={() => handlePresetClick(preset)}
                className={`rounded border px-3 py-1 transition-colors ${
                  isActive
                    ? 'border-emerald-600 bg-emerald-600 text-white'
                    : 'border-slate-300 text-slate-600 hover:bg-slate-100'
                } ${disabled ? 'opacity-60' : ''}`}
                disabled={disabled}
              >
                {preset.label}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
