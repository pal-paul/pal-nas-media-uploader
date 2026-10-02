import { Check, Pencil, X } from 'lucide-react'
import { useEffect } from 'react'
import type { Album, MediaItem } from '../types/gallery'
import { AuthenticatedImage } from './AuthenticatedMedia'

type Props = {
  open: boolean
  album: Album
  media: MediaItem[]
  name: string
  description: string
  coverMediaId?: string
  onNameChange: (value: string) => void
  onDescriptionChange: (value: string) => void
  onCoverChange: (id: string) => void
  onClose: () => void
  onSave: () => void
}

export function EditAlbumDialog({ open, album, media, name, description, coverMediaId, onNameChange, onDescriptionChange, onCoverChange, onClose, onSave }: Props) {
  useEffect(() => {
    if (!open) return
    const closeOnEscape = (event: KeyboardEvent) => event.key === 'Escape' && onClose()
    document.addEventListener('keydown', closeOnEscape)
    return () => document.removeEventListener('keydown', closeOnEscape)
  }, [onClose, open])

  if (!open) return null
  const photos = media.filter((item) => album.mediaIds.includes(item.id) && item.kind === 'photo')

  return (
    <div className="modal-backdrop" onMouseDown={onClose}>
      <section className="modal edit-album-modal" role="dialog" aria-modal="true" aria-labelledby="edit-album-title" onMouseDown={(event) => event.stopPropagation()}>
        <header><div className="modal-icon"><Pencil size={20} /></div><button className="icon-button" onClick={onClose} title="Close"><X size={19} /></button></header>
        <h2 id="edit-album-title">Edit album</h2>
        <label className="field-label">Album title<input autoFocus value={name} onChange={(event) => onNameChange(event.target.value)} onKeyDown={(event) => event.key === 'Enter' && onSave()} /></label>
        <label className="field-label">Description<textarea rows={3} maxLength={240} value={description} onChange={(event) => onDescriptionChange(event.target.value)} placeholder="Add a short description" /></label>
        <fieldset className="cover-picker">
          <legend>Cover image</legend>
          {photos.length > 0
            ? <div className="cover-options">{photos.map((item) => (
                <button className={coverMediaId === item.id ? 'selected' : ''} type="button" key={item.id} onClick={() => onCoverChange(item.id)} aria-label={`Use ${item.title} as cover`} aria-pressed={coverMediaId === item.id}>
                  <AuthenticatedImage src={item.url} alt="" />
                  {coverMediaId === item.id && <span><Check size={15} /></span>}
                </button>
              ))}</div>
            : <p className="cover-empty">Add a photo to this album to choose a cover.</p>}
        </fieldset>
        <footer><button className="secondary-button" onClick={onClose}>Cancel</button><button className="primary-button" disabled={!name.trim()} onClick={onSave}>Save changes</button></footer>
      </section>
    </div>
  )
}
