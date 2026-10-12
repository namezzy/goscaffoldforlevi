import { Modal } from './Modal'
import { Button } from './ui'

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
      <p className="mb-6 text-sm leading-relaxed text-slate-300">{message}</p>
      <div className="flex justify-end gap-2">
        <Button onClick={onCancel}>取消</Button>
        <Button variant={danger ? 'danger' : 'primary'} onClick={onConfirm} disabled={busy || disabled}>
          {busy ? '处理中…' : confirmLabel}
        </Button>
      </div>
    </Modal>
  )
}
