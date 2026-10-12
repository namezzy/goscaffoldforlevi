import type { ButtonHTMLAttributes, ReactNode } from 'react'
import { Link, type LinkProps } from 'react-router-dom'
import { ErrorBanner } from '../ErrorBanner'
import {
  buttonClass,
  cardClass,
  labelTextClass,
  toneClass,
  toneDotClass,
  type ButtonSize,
  type ButtonVariant,
  type Tone,
} from './styles'

export function Button({
  variant = 'secondary',
  size = 'md',
  className = '',
  type = 'button',
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: ButtonVariant; size?: ButtonSize }) {
  return <button type={type} className={buttonClass(variant, size, className)} {...rest} />
}

export function ButtonLink({
  variant = 'secondary',
  size = 'md',
  className = '',
  ...rest
}: LinkProps & { variant?: ButtonVariant; size?: ButtonSize }) {
  return <Link className={buttonClass(variant, size, className)} {...rest} />
}

/** The page's single heading row. Every page starts with one, so titles,
 * descriptions and primary actions always sit in the same place. */
export function PageHeader({
  title,
  description,
  actions,
  eyebrow,
}: {
  title: ReactNode
  description?: ReactNode
  actions?: ReactNode
  eyebrow?: ReactNode
}) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-4">
      <div className="min-w-0">
        {eyebrow && <div className="mb-1 font-mono text-[11px] uppercase tracking-[0.2em] text-cyan-400/80">{eyebrow}</div>}
        <h1 className="truncate text-xl font-semibold tracking-tight text-slate-50">{title}</h1>
        {description && <p className="mt-1 text-sm text-slate-400">{description}</p>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  )
}

export function Card({
  title,
  actions,
  children,
  className = '',
  padded = true,
}: {
  title?: ReactNode
  actions?: ReactNode
  children: ReactNode
  className?: string
  padded?: boolean
}) {
  return (
    <section className={`min-w-0 ${cardClass} ${className}`}>
      {(title || actions) && (
        <div className="flex items-center justify-between gap-3 border-b border-white/[0.06] px-5 py-3">
          {title && (
            <h2 className="flex items-center gap-2 text-sm font-semibold text-slate-100">
              <span className="h-3.5 w-0.5 rounded-full bg-cyan-400 shadow-[0_0_8px_rgba(34,211,238,0.8)]" />
              {title}
            </h2>
          )}
          {actions && <div className="flex items-center gap-2">{actions}</div>}
        </div>
      )}
      <div className={padded ? 'p-5' : ''}>{children}</div>
    </section>
  )
}

export function Badge({ tone = 'neutral', dot, children }: { tone?: Tone; dot?: boolean; children: ReactNode }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 whitespace-nowrap rounded-md border px-1.5 py-0.5 text-xs font-medium ${toneClass[tone]}`}
    >
      {dot && <span className={`h-1.5 w-1.5 rounded-full ${toneDotClass[tone]}`} />}
      {children}
    </span>
  )
}

/** A labelled single control. The label wraps its child, so clicking the
 * caption focuses the control — use FieldGroup when there is more than one. */
export function Field({ label, hint, children }: { label: string; hint?: ReactNode; children: ReactNode }) {
  return (
    <label className="block">
      <span className={labelTextClass}>{label}</span>
      {children}
      {hint && <span className="mt-1 block text-xs text-slate-500">{hint}</span>}
    </label>
  )
}

/** Field's look for a group of controls. Not a <label>: a label activates
 * its first labelable child, so wrapping several buttons in one would make
 * the caption a hidden click target for whichever comes first. */
export function FieldGroup({ id, label, children }: { id: string; label: string; children: ReactNode }) {
  return (
    <div role="group" aria-labelledby={id}>
      <span id={id} className={labelTextClass}>
        {label}
      </span>
      {children}
    </div>
  )
}

export function StatCard({
  label,
  value,
  hint,
  to,
  tone = 'cyan',
}: {
  label: ReactNode
  value: ReactNode
  hint?: ReactNode
  to?: string
  tone?: Tone
}) {
  const body = (
    <>
      <div className="flex items-center justify-between">
        <span className="font-mono text-[11px] uppercase tracking-wider text-slate-400">{label}</span>
        <span className={`h-1.5 w-1.5 rounded-full ${toneDotClass[tone]} shadow-[0_0_8px_currentColor]`} />
      </div>
      <div className="mt-2 font-mono text-3xl font-semibold tabular-nums text-slate-50">{value}</div>
      {hint && <div className="mt-1 text-xs text-slate-500">{hint}</div>}
    </>
  )
  const cls = `${cardClass} block p-4 transition`
  return to ? (
    <Link to={to} className={`${cls} hover:border-cyan-400/30 hover:bg-slate-900/80`}>
      {body}
    </Link>
  ) : (
    <div className={cls}>{body}</div>
  )
}

export function Loading({ label = '加载中…' }: { label?: string }) {
  return (
    <div className="flex items-center gap-2 py-6 text-sm text-slate-400">
      <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-cyan-400/30 border-t-cyan-400" />
      {label}
    </div>
  )
}

export function EmptyState({ children }: { children: ReactNode }) {
  return <p className="py-8 text-center text-sm text-slate-500">{children}</p>
}

export function ErrorState({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  return (
    <div className="space-y-3">
      <ErrorBanner error={error} />
      {onRetry && <Button onClick={onRetry}>重试</Button>}
    </div>
  )
}

export function Table({ children, minWidth }: { children: ReactNode; minWidth?: string }) {
  return (
    <div className={`${cardClass} overflow-hidden`}>
      <div className="overflow-x-auto">
        <table className="w-full table-fixed text-left text-sm" style={minWidth ? { minWidth } : undefined}>
          {children}
        </table>
      </div>
    </div>
  )
}

export function THead({ children }: { children: ReactNode }) {
  return (
    <thead className="border-b border-white/[0.06] bg-white/[0.02] font-mono text-[11px] uppercase tracking-wider text-slate-500">
      {children}
    </thead>
  )
}

export function TBody({ children }: { children: ReactNode }) {
  return <tbody className="divide-y divide-white/[0.04]">{children}</tbody>
}

export function Th({ children, className = '' }: { children?: ReactNode; className?: string }) {
  return <th className={`px-4 py-2.5 font-medium ${className}`}>{children}</th>
}

export function Tr({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <tr className={`transition hover:bg-cyan-400/[0.03] ${className}`}>{children}</tr>
}

export function Td({ children, className = '', colSpan }: { children?: ReactNode; className?: string; colSpan?: number }) {
  return (
    <td colSpan={colSpan} className={`px-4 py-2.5 text-slate-300 ${className}`}>
      {children}
    </td>
  )
}

/** A full-width row for a table's loading or empty state. */
export function TableMessage({ colSpan, children }: { colSpan: number; children: ReactNode }) {
  return (
    <tr>
      <td colSpan={colSpan} className="px-4 py-10 text-center text-sm text-slate-500">
        {children}
      </td>
    </tr>
  )
}

export function Pagination({
  page,
  totalPages,
  onChange,
}: {
  page: number
  totalPages: number
  onChange: (page: number) => void
}) {
  if (totalPages <= 1) return null
  return (
    <div className="flex items-center justify-center gap-3 text-sm">
      <Button size="sm" onClick={() => onChange(Math.max(page - 1, 1))} disabled={page <= 1}>
        上一页
      </Button>
      <span className="font-mono text-xs text-slate-400">
        第 {page} / {totalPages} 页
      </span>
      <Button size="sm" onClick={() => onChange(Math.min(page + 1, totalPages))} disabled={page >= totalPages}>
        下一页
      </Button>
    </div>
  )
}

/** A row of mutually exclusive options — page tabs, list scopes. */
export function SegmentedControl<T extends string>({
  options,
  value,
  onChange,
}: {
  options: { value: T; label: ReactNode }[]
  value: T
  onChange: (value: T) => void
}) {
  return (
    <div className="inline-flex flex-wrap gap-1 rounded-lg border border-white/[0.07] bg-slate-950/50 p-1">
      {options.map((o) => {
        const active = o.value === value
        return (
          <button
            key={o.value}
            type="button"
            aria-pressed={active}
            onClick={() => onChange(o.value)}
            className={`rounded-md px-3 py-1 text-sm transition ${
              active
                ? 'bg-cyan-400/15 text-cyan-200 shadow-[inset_0_0_0_1px_rgba(34,211,238,0.3)]'
                : 'text-slate-400 hover:text-slate-100'
            }`}
          >
            {o.label}
          </button>
        )
      })}
    </div>
  )
}

/** Label/value rows for a detail panel. */
export function DescriptionRow({ label, value, mono }: { label: ReactNode; value: ReactNode; mono?: boolean }) {
  return (
    <div className="flex justify-between gap-4 border-b border-white/[0.04] py-2 last:border-0">
      <dt className="shrink-0 text-sm text-slate-500">{label}</dt>
      <dd
        className={`min-w-0 truncate text-right text-slate-200 ${mono ? 'font-mono text-xs' : 'text-sm'}`}
        title={typeof value === 'string' ? value : undefined}
      >
        {value}
      </dd>
    </div>
  )
}
