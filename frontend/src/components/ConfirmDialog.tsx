import { Modal } from './Modal'

export function ConfirmDialog({
  title,
  message,
  confirmLabel = '确认',
  danger,
  onConfirm,
  onCancel,
  busy,
  disabled,
}: {
  title: string
  message: string
  confirmLabel?: string
  danger?: boolean
  onConfirm: () => void
  onCancel: () => void
  busy?: boolean
  /** Disables the confirm button without hiding it — used when the action is
   * known to fail server-side (e.g. deleting a department that still has
   * sub-departments), so the reason stays visible instead of the whole
   * dialog disappearing. */
  disabled?: boolean
}) {
  return (
    <Modal title={title} onClose={onCancel}>
      <p className="mb-6 text-sm text-neutral-600 dark:text-neutral-300">{message}</p>
      <div className="flex justify-end gap-2">
        <button
          onClick={onCancel}
          className="rounded-md border border-neutral-300 px-3 py-1.5 text-sm hover:bg-neutral-50 dark:border-neutral-700 dark:hover:bg-neutral-800"
        >
          取消
        </button>
        <button
          onClick={onConfirm}
          disabled={busy || disabled}
          className={`rounded-md px-3 py-1.5 text-sm font-medium text-white disabled:opacity-50 ${
            danger ? 'bg-red-600 hover:bg-red-700' : 'bg-neutral-900 hover:bg-neutral-800'
          }`}
        >
          {busy ? '处理中…' : confirmLabel}
        </button>
      </div>
    </Modal>
  )
}
