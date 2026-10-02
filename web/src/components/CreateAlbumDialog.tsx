import { FolderPlus, X } from 'lucide-react'

type Props = {
  open: boolean
  name: string
  onNameChange: (value: string) => void
  onClose: () => void
  onCreate: () => void
}

export function CreateAlbumDialog({ open, name, onNameChange, onClose, onCreate }: Props) {
  if (!open) return null
  return (
    <div className="modal-backdrop" onMouseDown={onClose}>
      <section className="modal" role="dialog" aria-modal="true" aria-labelledby="create-title" onMouseDown={(event) => event.stopPropagation()}>
        <header><div className="modal-icon"><FolderPlus size={21} /></div><button className="icon-button" onClick={onClose} title="Close"><X size={19} /></button></header>
        <h2 id="create-title">Create a new album</h2>
        <p>Give this collection a name. You can add photos and videos next.</p>
        <label className="field-label">Album name<input autoFocus value={name} onChange={(event) => onNameChange(event.target.value)} onKeyDown={(event) => event.key === 'Enter' && onCreate()} placeholder="e.g. Weekend in Copenhagen" /></label>
        <footer><button className="secondary-button" onClick={onClose}>Cancel</button><button className="primary-button" disabled={!name.trim()} onClick={onCreate}>Create album</button></footer>
      </section>
    </div>
  )
}
