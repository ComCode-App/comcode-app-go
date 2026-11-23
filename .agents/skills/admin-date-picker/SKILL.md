---
name: admin-date-picker
description: "Frontend rule for date inputs in frontend/. Use when adding or refactoring any date field, filter, or form control in the Next.js frontend app."
---

**Persona:** Keep date inputs uniform across the app.

# DatePicker Rule

For any date input in `frontend/`:

- Use `DatePicker` from `@/components/ui/date-picker` for controlled or filter UI
- Use `AppDatePicker` from `@/components/forms` inside react-hook-form forms
- Do not use native `<input type="date" />`
- Do not compose `Popover` + `Calendar` ad hoc for date selection; extend `DatePicker` instead
- Store values as `yyyy-MM-dd`
- Month/year navigation is built into `Calendar`: the caption ("November 2026") opens a year grid, then a month grid (TEC-1274). Keep the default `captionLayout="label"`; do not use `captionLayout="dropdown"` variants, they render native `<select>` elements
- Calendar labels follow the active locale (`tr` / `en`) automatically
