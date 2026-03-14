import {
  ResponsiveContainer,
  LineChart,
  Line,
  CartesianGrid,
  XAxis,
  YAxis,
  Tooltip,
} from 'recharts';

import { formatDateTimeShort } from '../utils/datetime';

type TelemetryChartDatum = {
  timestamp: string;
  value: number | null;
};

type TelemetryChartProps = {
  data: TelemetryChartDatum[];
  parameterLabel: string;
  unit?: string;
  isLoading?: boolean;
};

const NO_DATA_MESSAGE = 'No telemetry points available for the selected range.';

export function TelemetryChart({ data, parameterLabel, unit, isLoading }: TelemetryChartProps) {
  if (isLoading) {
    return <p className="text-sm text-slate-600">Loading chart…</p>;
  }

  if (!data.length || data.every((point) => point.value === null)) {
    return <p className="text-sm text-slate-600">{NO_DATA_MESSAGE}</p>;
  }

  return (
    <div className="h-72 w-full">
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data} margin={{ top: 12, right: 18, bottom: 12, left: 0 }}>
          <CartesianGrid stroke="#e2e8f0" strokeDasharray="4 2" />
          <XAxis
            dataKey="timestamp"
            stroke="#475569"
            tickFormatter={(value: string | number) => formatDateTimeShort(value)}
            minTickGap={36}
          />
          <YAxis
            stroke="#475569"
            allowDecimals
            domain={['auto', 'auto']}
            tickFormatter={(value: number) =>
              Number.isFinite(value)
                ? value.toLocaleString(undefined, { maximumFractionDigits: 2 })
                : ''
            }
            label={
              unit ? { value: unit, angle: -90, position: 'insideLeft', offset: 10 } : undefined
            }
          />
          <Tooltip
            formatter={(value: number | string | Array<number | string>) => {
              const numeric = Array.isArray(value)
                ? Number(value[0])
                : typeof value === 'string'
                  ? Number(value)
                  : value;
              if (typeof numeric === 'number' && Number.isFinite(numeric)) {
                const formatted = `${numeric.toLocaleString(undefined, { maximumFractionDigits: 2 })}${unit ? ` ${unit}` : ''}`;
                return [formatted, parameterLabel];
              }
              return ['N/A', parameterLabel];
            }}
            labelFormatter={(value: string | number) => formatDateTimeShort(value)}
          />
          <Line
            type="monotone"
            dataKey="value"
            stroke="#047857"
            strokeWidth={2}
            dot={false}
            connectNulls
            name={parameterLabel}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
