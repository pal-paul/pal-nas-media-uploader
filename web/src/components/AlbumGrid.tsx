import { ArrowDown, ArrowUp, CalendarDays, Image, Images } from 'lucide-react'
import type { CSSProperties } from 'react'
import type { Album } from '../types/gallery'
import { formatDate } from '../utils/date'
import { AuthenticatedImage } from './AuthenticatedMedia'

type Props = { albums: Album[]; onOpen: (id: string) => void; onMove: (id: string, offset: -1 | 1) => void }

export function AlbumGrid({ albums, onOpen, onMove }: Props) {
  return (
    <section className="album-grid" aria-live="polite">
      {albums.map((album, index) => {
        return (
          <article className={`album-card diagonal-${index % 6}`} key={album.id} style={{ '--delay': `${index * 45}ms` } as CSSProperties}>
            <button className="album-cover" onClick={() => onOpen(album.id)}>
              {album.coverUrl && <AuthenticatedImage src={album.coverUrl} alt={`${album.title} cover`} />}
              {!album.coverUrl && <span className="empty-cover"><Image size={28} /><small>No media yet</small></span>}
              <span className="album-meta">
                <span className="album-copy">
                  <strong>{album.title}</strong>
                  <span><CalendarDays size={13} /> {formatDate(album.createdAt)} <b>·</b> <Images size={13} /> {album.itemCount}</span>
                </span>
              </span>
            </button>
            <div className="album-order-controls" aria-label={`Reorder ${album.title}`}>
              <button onClick={() => onMove(album.id, -1)} disabled={index === 0} title="Move album earlier" aria-label={`Move ${album.title} earlier`}><ArrowUp size={14} /></button>
              <button onClick={() => onMove(album.id, 1)} disabled={index === albums.length - 1} title="Move album later" aria-label={`Move ${album.title} later`}><ArrowDown size={14} /></button>
            </div>
          </article>
        )
      })}
    </section>
  )
}
