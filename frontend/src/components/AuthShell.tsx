import type { ReactNode } from 'react'

/** The frame for every signed-out page — login, forgot/reset password,
 * accept invitation — so they share one look. */
export function AuthShell({ title, subtitle, children }: { title: string; subtitle?: ReactNode; children: ReactNode }) {
  return (
    <div className="flex min-h-screen items-center justify-center px-4 py-12">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center text-center">
          <span className="mb-4 flex h-11 w-11 items-center justify-center rounded-xl border border-cyan-400/40 bg-cyan-400/10 shadow-[0_0_30px_-4px_rgba(34,211,238,0.6)]">
            <span className="h-3 w-3 rounded-sm bg-cyan-300" />
          </span>
          <div className="font-mono text-[11px] uppercase tracking-[0.3em] text-cyan-400/80">goscaffold · levi</div>
          <h1 className="mt-2 text-2xl font-semibold tracking-tight text-slate-50">{title}</h1>
          {subtitle && <p className="mt-1.5 text-sm text-slate-400">{subtitle}</p>}
        </div>
        <div className="relative rounded-2xl border border-white/[0.08] bg-slate-900/60 p-7 shadow-[0_0_60px_-20px_rgba(34,211,238,0.4)] backdrop-blur-md">
          <div className="pointer-events-none absolute inset-x-8 -top-px h-px bg-gradient-to-r from-transparent via-cyan-400/70 to-transparent" />
          {children}
        </div>
      </div>
    </div>
  )
}
