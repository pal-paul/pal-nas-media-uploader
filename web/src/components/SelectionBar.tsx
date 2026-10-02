import { FolderPlus, X } from 'lucide-react'
import type { Album } from '../types/gallery'

type Props = {
  count: number
  albums: Album[]
  targetAlbumId: string
  onTargetChange: (id: string) => void
  onAdd: () => void
  onClear: () => void
}

export function SelectionBar({ count, albums, targetAlbumId, onTargetChange, onAdd, onClear }: Props) {
  if (count === 0) return null
  return (
    <section className="selection-bar">
      <strong>{count} selected</strong><span className="selection-divider" />
      <label>Add to <select value={targetAlbumId} onChange={(event) => onTargetChange(event.target.value)}>{albums.map((album) => <option key={album.id} value={album.id}>{album.title}</option>)}</select></label>
      <button className="primary-button compact" onClick={onAdd}><FolderPlus size={16} /> Add</button>
      <button className="icon-button" onClick={onClear} title="Clear selection"><X size={18} /></button>
    </section>
  )
}
