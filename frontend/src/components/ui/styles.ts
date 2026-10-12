// Class-name tokens shared by the UI primitives and the few places that
// style a native element directly (a <select> inside a filter bar, say).
// Kept apart from the components so Fast Refresh stays component-only.

/** Form control styling without a width — for inline filter bars. */
export const controlClass =
  'rounded-lg border border-white/10 bg-slate-950/60 px-3 py-2 text-sm text-slate-100 placeholder:text-slate-500 outline-none transition focus:border-cyan-400/60 focus:ring-2 focus:ring-cyan-400/20 disabled:opacity-50'

export const inputClass = `w-full ${controlClass}`

export const labelTextClass = 'mb-1.5 block text-xs font-medium tracking-wide text-slate-400'

export const linkClass = 'text-cyan-300 transition hover:text-cyan-200 hover:underline'

export type Tone = 'neutral' | 'cyan' | 'green' | 'amber' | 'red' | 'blue' | 'violet'

export const toneClass: Record<Tone, string> = {
  neutral: 'border-slate-500/30 bg-slate-500/10 text-slate-300',
  cyan: 'border-cyan-400/30 bg-cyan-400/10 text-cyan-300',
  green: 'border-emerald-400/30 bg-emerald-400/10 text-emerald-300',
  amber: 'border-amber-400/30 bg-amber-400/10 text-amber-300',
  red: 'border-rose-400/30 bg-rose-400/10 text-rose-300',
  blue: 'border-sky-400/30 bg-sky-400/10 text-sky-300',
  violet: 'border-violet-400/30 bg-violet-400/10 text-violet-300',
}

export const toneDotClass: Record<Tone, string> = {
  neutral: 'bg-slate-400',
  cyan: 'bg-cyan-400',
  green: 'bg-emerald-400',
  amber: 'bg-amber-400',
  red: 'bg-rose-400',
  blue: 'bg-sky-400',
  violet: 'bg-violet-400',
}

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'success'
export type ButtonSize = 'sm' | 'md'

const buttonBase =
  'inline-flex items-center justify-center gap-1.5 whitespace-nowrap rounded-lg font-medium transition outline-none focus-visible:ring-2 focus-visible:ring-cyan-400/50 disabled:pointer-events-none disabled:opacity-40'

const buttonVariant: Record<ButtonVariant, string> = {
  primary:
    'bg-gradient-to-r from-cyan-500 to-sky-500 text-slate-950 shadow-[0_0_18px_-4px_rgba(34,211,238,0.6)] hover:from-cyan-400 hover:to-sky-400',
  secondary: 'border border-white/10 bg-white/[0.03] text-slate-200 hover:border-cyan-400/40 hover:bg-cyan-400/5 hover:text-cyan-200',
  ghost: 'text-slate-400 hover:bg-white/5 hover:text-slate-100',
  danger: 'border border-rose-400/30 bg-rose-500/10 text-rose-300 hover:border-rose-400/60 hover:bg-rose-500/20',
  success: 'border border-emerald-400/30 bg-emerald-500/10 text-emerald-300 hover:border-emerald-400/60 hover:bg-emerald-500/20',
}

const buttonSize: Record<ButtonSize, string> = {
  sm: 'px-2.5 py-1 text-xs',
  md: 'px-3.5 py-2 text-sm',
}

export function buttonClass(variant: ButtonVariant = 'secondary', size: ButtonSize = 'md', extra = ''): string {
  return `${buttonBase} ${buttonVariant[variant]} ${buttonSize[size]} ${extra}`.trim()
}

export const cardClass = 'rounded-xl border border-white/[0.07] bg-slate-900/50 backdrop-blur-sm'

export function formatDateTime(ts: string | undefined | null): string {
  return ts ? new Date(ts).toLocaleString('zh-CN', { hour12: false }) : '—'
}
