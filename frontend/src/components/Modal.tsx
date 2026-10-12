import { useEffect, useRef, type ReactNode } from 'react'

const FOCUSABLE_SELECTOR =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'

export function Modal({
  title,
  onClose,
  children,
}: {
  title: string
  onClose: () => void
  children: ReactNode
}) {
  const dialogRef = useRef<HTMLDivElement>(null)
  const bodyRef = useRef<HTMLDivElement>(null)
  const onCloseRef = useRef(onClose)
  // Kept current via an effect rather than assigned inline during render:
  // mutating a ref while rendering runs on every render pass React
  // attempts, including ones it discards without committing, and this
  // value should only ever reflect the render that actually landed.
  useEffect(() => {
    onCloseRef.current = onClose
  })

  // Focus enters the dialog on open and returns to whatever opened it on
  // close — a mouse user never notices, but without this a keyboard or
  // screen-reader user loses their place every time a modal closes.
  //
  // The target is the first focusable element in the body, not the dialog
  // as a whole: the header's own close button would otherwise win by
  // appearing first in DOM order, landing every dialog on "✕" instead of
  // the field or action someone actually opened it to use.
  useEffect(() => {
    const trigger = document.activeElement as HTMLElement | null
    const first = bodyRef.current?.querySelector<HTMLElement>(FOCUSABLE_SELECTOR)
    ;(first ?? dialogRef.current)?.focus()

    return () => {
      trigger?.focus()
    }
  }, [])

  // Escape closes, and Tab is trapped inside the dialog rather than
  // escaping into the page behind it — a background full of disabled-
  // looking controls that a keyboard user could still reach would be worse
  // than the overlay not being there at all.
  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        e.stopPropagation()
        onCloseRef.current()
        return
      }
      if (e.key !== 'Tab') return

      const dialog = dialogRef.current
      if (!dialog) return
      const focusable = Array.from(dialog.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR))
      if (focusable.length === 0) {
        e.preventDefault()
        return
      }
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      const active = document.activeElement

      if (e.shiftKey && active === first) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && active === last) {
        e.preventDefault()
        first.focus()
      }
    }

    document.addEventListener('keydown', onKeyDown, true)
    return () => document.removeEventListener('keydown', onKeyDown, true)
  }, [])

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
      <div
        ref={dialogRef}
        className="flex max-h-[85vh] w-full max-w-md flex-col rounded-lg bg-white shadow-xl outline-none dark:bg-neutral-900"
        role="dialog"
        aria-modal="true"
        aria-label={title}
        tabIndex={-1}
      >
        <div className="flex shrink-0 items-center justify-between border-b border-neutral-100 p-6 pb-4 dark:border-neutral-800">
          <h2 className="text-lg font-semibold text-neutral-900 dark:text-neutral-100">{title}</h2>
          <button
            onClick={onClose}
            className="text-neutral-400 hover:text-neutral-600 dark:hover:text-neutral-200"
            aria-label="关闭"
          >
            ✕
          </button>
        </div>
        <div ref={bodyRef} className="overflow-y-auto p-6 pt-4">
          {children}
        </div>
      </div>
    </div>
  )
}
