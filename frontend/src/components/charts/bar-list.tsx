export type BarListItem = { key: string; label: string; count: number };

/** Horizontal magnitude bars (one hue), value label in text ink, never color alone. */
export function BarList({
  items,
  format,
  limit = 10,
}: {
  items: BarListItem[];
  format: (n: number) => string;
  limit?: number;
}) {
  const max = Math.max(1, ...items.map((i) => i.count));
  return (
    <ul className="space-y-2">
      {items.slice(0, limit).map((item) => (
        <li key={item.key || item.label} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1">
          <span className="truncate text-sm">{item.label}</span>
          <span className="text-sm tabular-nums">{format(item.count)}</span>
          <span className="bg-muted col-span-2 h-2 overflow-hidden rounded-full">
            <span
              className="block h-full rounded-full"
              style={{ width: `${(item.count / max) * 100}%`, backgroundColor: "var(--chart-1)" }}
            />
          </span>
        </li>
      ))}
    </ul>
  );
}
