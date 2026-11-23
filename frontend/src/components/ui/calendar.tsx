"use client";

import * as React from "react";
import {
  ChevronDownIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
} from "lucide-react";
import {
  DayPicker,
  getDefaultClassNames,
  type DayButton,
  type MonthCaptionProps,
} from "react-day-picker";

import { cn } from "@/lib/utils/index";
import { Button, buttonVariants } from "@/components/ui/button";

type CalendarView = "days" | "months" | "years";

/** Years shown per page in the year grid (3 columns x 4 rows). */
const YEARS_PER_PAGE = 12;

const PERIOD_LABELS = {
  tr: {
    choosePeriod: "Ay ve yıl seçin",
    previousYear: "Önceki yıl",
    nextYear: "Sonraki yıl",
    previousYears: "Önceki yıllar",
    nextYears: "Sonraki yıllar",
    chooseYear: "Yıl seçin",
  },
  en: {
    choosePeriod: "Choose month and year",
    previousYear: "Previous year",
    nextYear: "Next year",
    previousYears: "Previous years",
    nextYears: "Next years",
    chooseYear: "Choose year",
  },
} as const;

function periodLabels(code: string | undefined) {
  return code?.toLowerCase().startsWith("tr")
    ? PERIOD_LABELS.tr
    : PERIOD_LABELS.en;
}

function firstSelectedDate(selected: unknown): Date | undefined {
  if (selected instanceof Date) return selected;
  if (Array.isArray(selected) && selected[0] instanceof Date)
    return selected[0];
  if (selected && typeof selected === "object" && "from" in selected) {
    const from = (selected as { from?: unknown }).from;
    if (from instanceof Date) return from;
  }
  return undefined;
}

const monthIndex = (d: Date) => d.getFullYear() * 12 + d.getMonth();

/**
 * Shared calendar. With the default `captionLayout="label"` the month caption is a button:
 * it opens a year grid, then a month grid, so far-away dates (birthdays, old records) are a
 * few clicks away. No native `<select>` is involved (react-day-picker's "dropdown" layouts
 * render native selects; avoid them). The year/month panel keeps the day grid's exact size.
 */
function Calendar({
  className,
  classNames,
  showOutsideDays = true,
  captionLayout = "label",
  buttonVariant = "ghost",
  formatters,
  components,
  ...props
}: React.ComponentProps<typeof DayPicker> & {
  buttonVariant?: React.ComponentProps<typeof Button>["variant"];
}) {
  const defaultClassNames = getDefaultClassNames();
  const rootClassName = cn(
    "group/calendar bg-background p-3 [--cell-size:--spacing(8)] [[data-slot=card-content]_&]:bg-transparent [[data-slot=popover-content]_&]:bg-transparent",
    String.raw`rtl:**:[.rdp-button\_next>svg]:rotate-180`,
    String.raw`rtl:**:[.rdp-button\_previous>svg]:rotate-180`,
    className,
  );

  const periodPicker = captionLayout === "label";
  const [innerMonth, setInnerMonth] = React.useState<Date>(
    () =>
      props.month ??
      props.defaultMonth ??
      firstSelectedDate((props as { selected?: unknown }).selected) ??
      props.today ??
      new Date(),
  );
  const month = props.month ?? innerMonth;
  const { onMonthChange } = props;
  const changeMonth = React.useCallback(
    (next: Date) => {
      setInnerMonth(next);
      onMonthChange?.(next);
    },
    [onMonthChange],
  );

  const [view, setView] = React.useState<CalendarView>("days");
  // The first month of the page being changed (multi-month calendars).
  const [offset, setOffset] = React.useState(0);
  const [panelYear, setPanelYear] = React.useState(month.getFullYear());
  const [size, setSize] = React.useState<{ width: number; height: number }>();
  const boxRef = React.useRef<HTMLDivElement>(null);

  const openPanel = React.useCallback(
    (displayMonth: Date, displayIndex: number) => {
      const box = boxRef.current?.firstElementChild as HTMLElement | null;
      if (box) setSize({ width: box.offsetWidth, height: box.offsetHeight });
      setOffset(displayIndex);
      setPanelYear(displayMonth.getFullYear());
      setView("years");
    },
    [],
  );

  const labels = periodLabels(props.locale?.code);
  const PeriodCaption = React.useMemo(() => {
    function PeriodCaption({
      calendarMonth,
      displayIndex,
      children,
      ...divProps
    }: MonthCaptionProps) {
      return (
        <div {...divProps}>
          <button
            type="button"
            aria-label={labels.choosePeriod}
            onClick={() => openPanel(calendarMonth.date, displayIndex)}
            className="hover:bg-accent hover:text-accent-foreground focus-visible:border-ring focus-visible:ring-ring/50 relative z-10 inline-flex h-7 items-center gap-1 rounded-md px-2 outline-none focus-visible:ring-[3px]"
          >
            {children}
            <ChevronDownIcon
              aria-hidden
              className="text-muted-foreground size-3.5"
            />
          </button>
        </div>
      );
    }
    return PeriodCaption;
  }, [labels, openPanel]);

  if (periodPicker && view !== "days") {
    return (
      <div ref={boxRef}>
        <PeriodPanel
          className={rootClassName}
          size={size}
          view={view}
          year={panelYear}
          month={month}
          selected={firstSelectedDate(
            (props as { selected?: unknown }).selected,
          )}
          today={props.today ?? new Date()}
          startMonth={props.startMonth}
          endMonth={props.endMonth}
          locale={props.locale}
          labels={labels}
          buttonVariant={buttonVariant}
          onYearPage={setPanelYear}
          onPickYear={(year) => {
            setPanelYear(year);
            setView("months");
          }}
          onShowYears={() => setView("years")}
          onPickMonth={(year, m) => {
            changeMonth(new Date(year, m - offset, 1));
            setView("days");
          }}
        />
      </div>
    );
  }

  return (
    <div ref={boxRef} className="contents">
      <DayPicker
        showOutsideDays={showOutsideDays}
        className={rootClassName}
        captionLayout={captionLayout}
        formatters={{
          formatMonthDropdown: (date) =>
            date.toLocaleString(props.locale?.code ?? "default", {
              month: "short",
            }),
          ...formatters,
        }}
        classNames={{
          root: cn("w-fit", defaultClassNames.root),
          months: cn(
            "relative flex flex-col gap-4 md:flex-row",
            defaultClassNames.months,
          ),
          month: cn("flex w-full flex-col gap-4", defaultClassNames.month),
          nav: cn(
            "absolute inset-x-0 top-0 flex w-full items-center justify-between gap-1",
            defaultClassNames.nav,
          ),
          button_previous: cn(
            buttonVariants({ variant: buttonVariant }),
            "size-(--cell-size) p-0 select-none aria-disabled:opacity-50",
            defaultClassNames.button_previous,
          ),
          button_next: cn(
            buttonVariants({ variant: buttonVariant }),
            "size-(--cell-size) p-0 select-none aria-disabled:opacity-50",
            defaultClassNames.button_next,
          ),
          month_caption: cn(
            "flex h-(--cell-size) w-full items-center justify-center px-(--cell-size)",
            defaultClassNames.month_caption,
          ),
          dropdowns: cn(
            "flex h-(--cell-size) w-full items-center justify-center gap-1.5 text-sm font-medium",
            defaultClassNames.dropdowns,
          ),
          dropdown_root: cn(
            "relative rounded-md border border-input shadow-xs has-focus:border-ring has-focus:ring-[3px] has-focus:ring-ring/50",
            defaultClassNames.dropdown_root,
          ),
          dropdown: cn(
            "absolute inset-0 bg-popover opacity-0",
            defaultClassNames.dropdown,
          ),
          caption_label: cn(
            "font-medium select-none",
            captionLayout === "label"
              ? "text-sm"
              : "flex h-8 items-center gap-1 rounded-md pr-1 pl-2 text-sm [&>svg]:size-3.5 [&>svg]:text-muted-foreground",
            defaultClassNames.caption_label,
          ),
          month_grid: cn(
            "w-full border-collapse",
            defaultClassNames.month_grid,
          ),
          weekdays: cn("flex", defaultClassNames.weekdays),
          weekday: cn(
            "flex-1 rounded-md text-[0.8rem] font-normal text-muted-foreground select-none",
            defaultClassNames.weekday,
          ),
          week: cn("mt-2 flex w-full", defaultClassNames.week),
          week_number_header: cn(
            "w-(--cell-size) select-none",
            defaultClassNames.week_number_header,
          ),
          week_number: cn(
            "text-[0.8rem] text-muted-foreground select-none",
            defaultClassNames.week_number,
          ),
          day: cn(
            "group/day relative aspect-square h-full w-full p-0 text-center select-none [&:last-child[data-selected=true]_button]:rounded-r-md",
            props.showWeekNumber
              ? "[&:nth-child(2)[data-selected=true]_button]:rounded-l-md"
              : "[&:first-child[data-selected=true]_button]:rounded-l-md",
            defaultClassNames.day,
          ),
          range_start: cn(
            "rounded-l-md bg-accent",
            defaultClassNames.range_start,
          ),
          range_middle: cn("rounded-none", defaultClassNames.range_middle),
          range_end: cn("rounded-r-md bg-accent", defaultClassNames.range_end),
          today: cn(
            "rounded-md bg-accent text-accent-foreground data-[selected=true]:rounded-none",
            defaultClassNames.today,
          ),
          outside: cn(
            "text-muted-foreground aria-selected:text-muted-foreground",
            defaultClassNames.outside,
          ),
          disabled: cn(
            "text-muted-foreground opacity-50",
            defaultClassNames.disabled,
          ),
          hidden: cn("invisible", defaultClassNames.hidden),
          ...classNames,
        }}
        components={{
          Root: ({ className, rootRef, ...props }) => {
            return (
              <div
                data-slot="calendar"
                ref={rootRef}
                className={cn(className)}
                {...props}
              />
            );
          },
          Chevron: ({ className, orientation, ...props }) => {
            if (orientation === "left") {
              return (
                <ChevronLeftIcon
                  className={cn("size-4", className)}
                  {...props}
                />
              );
            }

            if (orientation === "right") {
              return (
                <ChevronRightIcon
                  className={cn("size-4", className)}
                  {...props}
                />
              );
            }

            return (
              <ChevronDownIcon className={cn("size-4", className)} {...props} />
            );
          },
          DayButton: CalendarDayButton,
          WeekNumber: ({ children, ...props }) => {
            return (
              <td {...props}>
                <div className="flex size-(--cell-size) items-center justify-center text-center">
                  {children}
                </div>
              </td>
            );
          },
          ...(periodPicker ? { MonthCaption: PeriodCaption } : {}),
          ...components,
        }}
        {...props}
        month={month}
        onMonthChange={changeMonth}
      />
    </div>
  );
}

type PeriodPanelProps = {
  className: string;
  size?: { width: number; height: number };
  view: Exclude<CalendarView, "days">;
  year: number;
  month: Date;
  selected?: Date;
  today: Date;
  startMonth?: Date;
  endMonth?: Date;
  locale?: React.ComponentProps<typeof DayPicker>["locale"];
  labels: (typeof PERIOD_LABELS)[keyof typeof PERIOD_LABELS];
  buttonVariant: React.ComponentProps<typeof Button>["variant"];
  onYearPage: (year: number) => void;
  onPickYear: (year: number) => void;
  onShowYears: () => void;
  onPickMonth: (year: number, month: number) => void;
};

/** Year grid (12 per page) and month grid that replace the day grid at the same size. */
function PeriodPanel({
  className,
  size,
  view,
  year,
  month,
  selected,
  today,
  startMonth,
  endMonth,
  locale,
  labels,
  buttonVariant,
  onYearPage,
  onPickYear,
  onShowYears,
  onPickMonth,
}: PeriodPanelProps) {
  const minYear = startMonth?.getFullYear() ?? -Infinity;
  const maxYear = endMonth?.getFullYear() ?? Infinity;
  const pageStart = Math.floor(year / YEARS_PER_PAGE) * YEARS_PER_PAGE;
  const pageEnd = pageStart + YEARS_PER_PAGE - 1;
  const navClass = cn(
    buttonVariants({ variant: buttonVariant }),
    "size-(--cell-size) p-0 select-none disabled:opacity-50",
  );
  const activeYear = selected?.getFullYear() ?? month.getFullYear();
  const activeMonth = selected ?? month;

  const prev =
    view === "years"
      ? {
          label: labels.previousYears,
          to: year - YEARS_PER_PAGE,
          off: pageStart - 1 < minYear,
        }
      : { label: labels.previousYear, to: year - 1, off: year - 1 < minYear };
  const next =
    view === "years"
      ? {
          label: labels.nextYears,
          to: year + YEARS_PER_PAGE,
          off: pageEnd + 1 > maxYear,
        }
      : { label: labels.nextYear, to: year + 1, off: year + 1 > maxYear };

  const cells =
    view === "years"
      ? Array.from({ length: YEARS_PER_PAGE }, (_, i) => {
          const y = pageStart + i;
          return {
            key: y,
            text: String(y),
            active: y === activeYear,
            current: y === today.getFullYear(),
            disabled: y < minYear || y > maxYear,
            pick: () => onPickYear(y),
          };
        })
      : Array.from({ length: 12 }, (_, m) => {
          const at = new Date(year, m, 1);
          return {
            key: m,
            text: at.toLocaleString(locale?.code ?? "default", {
              month: "short",
            }),
            active:
              year === activeMonth.getFullYear() &&
              m === activeMonth.getMonth(),
            current: year === today.getFullYear() && m === today.getMonth(),
            disabled:
              (startMonth !== undefined &&
                monthIndex(at) < monthIndex(startMonth)) ||
              (endMonth !== undefined && monthIndex(at) > monthIndex(endMonth)),
            pick: () => onPickMonth(year, m),
          };
        });
  const focusKey = (
    cells.find((c) => c.active && !c.disabled) ?? cells.find((c) => !c.disabled)
  )?.key;

  return (
    <div
      data-slot="calendar"
      data-view={view}
      className={cn(className, "flex flex-col gap-4")}
      style={size ? { width: size.width, height: size.height } : undefined}
    >
      <div className="flex h-(--cell-size) items-center justify-between gap-1">
        <button
          type="button"
          className={navClass}
          aria-label={prev.label}
          disabled={prev.off}
          onClick={() => onYearPage(prev.to)}
        >
          <ChevronLeftIcon className="size-4" />
        </button>
        {view === "months" ? (
          <button
            type="button"
            aria-label={labels.chooseYear}
            onClick={onShowYears}
            className="hover:bg-accent hover:text-accent-foreground focus-visible:border-ring focus-visible:ring-ring/50 inline-flex h-7 items-center gap-1 rounded-md px-2 text-sm font-medium outline-none select-none focus-visible:ring-[3px]"
          >
            {year}
            <ChevronDownIcon
              aria-hidden
              className="text-muted-foreground size-3.5"
            />
          </button>
        ) : (
          <span className="text-sm font-medium select-none" aria-live="polite">
            {pageStart} – {pageEnd}
          </span>
        )}
        <button
          type="button"
          className={navClass}
          aria-label={next.label}
          disabled={next.off}
          onClick={() => onYearPage(next.to)}
        >
          <ChevronRightIcon className="size-4" />
        </button>
      </div>
      <div className="grid min-w-[calc(var(--cell-size)*7)] flex-1 auto-rows-fr grid-cols-3 gap-1.5">
        {cells.map((cell) => (
          <Button
            key={cell.key}
            type="button"
            variant={cell.active ? "default" : "ghost"}
            disabled={cell.disabled}
            aria-pressed={cell.active}
            autoFocus={cell.key === focusKey}
            onClick={cell.pick}
            className={cn(
              "h-full min-h-(--cell-size) w-full px-0 font-normal capitalize",
              cell.current &&
                !cell.active &&
                "bg-accent text-accent-foreground",
            )}
          >
            {cell.text}
          </Button>
        ))}
      </div>
    </div>
  );
}

function CalendarDayButton({
  className,
  day,
  modifiers,
  ...props
}: React.ComponentProps<typeof DayButton>) {
  const defaultClassNames = getDefaultClassNames();

  const ref = React.useRef<HTMLButtonElement>(null);
  React.useEffect(() => {
    if (modifiers.focused) ref.current?.focus();
  }, [modifiers.focused]);

  return (
    <Button
      ref={ref}
      variant="ghost"
      size="icon"
      data-day={day.date.toLocaleDateString()}
      data-selected-single={
        modifiers.selected &&
        !modifiers.range_start &&
        !modifiers.range_end &&
        !modifiers.range_middle
      }
      data-range-start={modifiers.range_start}
      data-range-end={modifiers.range_end}
      data-range-middle={modifiers.range_middle}
      className={cn(
        "group-data-[focused=true]/day:border-ring group-data-[focused=true]/day:ring-ring/50 data-[range-end=true]:bg-primary data-[range-end=true]:text-primary-foreground data-[range-middle=true]:bg-accent data-[range-middle=true]:text-accent-foreground data-[range-start=true]:bg-primary data-[range-start=true]:text-primary-foreground data-[selected-single=true]:bg-primary data-[selected-single=true]:text-primary-foreground dark:hover:text-accent-foreground flex aspect-square size-auto w-full min-w-(--cell-size) flex-col gap-1 leading-none font-normal group-data-[focused=true]/day:relative group-data-[focused=true]/day:z-10 group-data-[focused=true]/day:ring-[3px] data-[range-end=true]:rounded-md data-[range-end=true]:rounded-r-md data-[range-middle=true]:rounded-none data-[range-start=true]:rounded-md data-[range-start=true]:rounded-l-md [&>span]:text-xs [&>span]:opacity-70",
        defaultClassNames.day,
        className,
      )}
      {...props}
    />
  );
}

export { Calendar, CalendarDayButton };
