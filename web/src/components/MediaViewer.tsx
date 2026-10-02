import { CalendarDays, Camera, ChevronLeft, ChevronRight, Clock3, FileImage, Info, MapPin, Ruler, Tags, X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import type { MediaItem } from '../types/gallery'
import { formatDate } from '../utils/date'
import { AuthenticatedImage, AuthenticatedVideo } from './AuthenticatedMedia'

type Props = {
  item: MediaItem
  hasMultiple: boolean
  onClose: () => void
  onPrevious: () => void
  onNext: () => void
}

export function MediaViewer({ item, hasMultiple, onClose, onPrevious, onNext }: Props) {
  const [showDetails, setShowDetails] = useState(false)
  const touchStart = useRef<{ x: number; y: number } | null>(null)

  const handleTouchStart = (event: React.TouchEvent) => {
    const touch = event.touches[0]
    touchStart.current = { x: touch.clientX, y: touch.clientY }
  }

  const handleTouchEnd = (event: React.TouchEvent) => {
    if (!hasMultiple || !touchStart.current) return
    const touch = event.changedTouches[0]
    const deltaX = touch.clientX - touchStart.current.x
    const deltaY = touch.clientY - touchStart.current.y
    touchStart.current = null
    if (Math.abs(deltaX) < 45 || Math.abs(deltaX) <= Math.abs(deltaY)) return
    if (deltaX > 0) onPrevious()
    else onNext()
  }

  useEffect(() => {
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
      if (event.key === 'ArrowLeft' && hasMultiple) onPrevious()
      if (event.key === 'ArrowRight' && hasMultiple) onNext()
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.body.style.overflow = previousOverflow
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [hasMultiple, onClose, onNext, onPrevious])

  return (
    <div className="media-viewer" role="dialog" aria-modal="true" aria-label={item.title} onClick={(event) => event.target === event.currentTarget && onClose()} onTouchStart={handleTouchStart} onTouchEnd={handleTouchEnd}>
      <button className={`viewer-info ${showDetails ? 'active' : ''}`} onClick={() => setShowDetails((visible) => !visible)} aria-label="Media details" title="Media details" aria-expanded={showDetails}><Info size={21} /></button>
      <button className="viewer-close" onClick={onClose} aria-label="Close viewer"><X size={22} /></button>
      {hasMultiple && !showDetails && <button className="viewer-nav viewer-previous" onClick={onPrevious} aria-label="Previous media"><ChevronLeft size={28} /></button>}
      <figure className="viewer-content" key={item.id}>
        {item.kind === 'video'
          ? <AuthenticatedVideo key={item.id} src={item.url} controls autoPlay />
          : <AuthenticatedImage src={item.url} alt={item.title} />}
        <figcaption><strong>{item.title}</strong></figcaption>
      </figure>
      {showDetails && (
        <aside className="viewer-details" aria-label="Media metadata">
          <h2>Details</h2>
          <dl>
            <div><dt><CalendarDays size={16} /> Date</dt><dd>{formatDate(item.createdAt)}</dd></div>
            <div><dt><FileImage size={16} /> Type</dt><dd>{item.kind === 'photo' ? 'Photo' : 'Video'}</dd></div>
            {item.duration && <div><dt><Clock3 size={16} /> Duration</dt><dd>{item.duration}</dd></div>}
            {item.path && <div><dt><FileImage size={16} /> File</dt><dd>{item.path.split(/[/\\]/).pop()}</dd></div>}
            {item.fileSize != null && <div><dt><FileImage size={16} /> Size</dt><dd>{new Intl.NumberFormat(undefined, { style: 'unit', unit: 'megabyte', maximumFractionDigits: 1 }).format(item.fileSize / 1_000_000)}</dd></div>}
            {item.width && item.height && <div><dt><Ruler size={16} /> Dimensions</dt><dd>{item.width} × {item.height}</dd></div>}
            {item.camera && <div><dt><Camera size={16} /> Camera</dt><dd>{item.camera}</dd></div>}
            {item.latitude != null && item.longitude != null && <div><dt><MapPin size={16} /> Location</dt><dd>{item.latitude.toFixed(5)}, {item.longitude.toFixed(5)}</dd></div>}
            {item.tags.length > 0 && <div><dt><Tags size={16} /> Tags</dt><dd>{item.tags.join(', ')}</dd></div>}
          </dl>
        </aside>
      )}
      {hasMultiple && !showDetails && <button className="viewer-nav viewer-next" onClick={onNext} aria-label="Next media"><ChevronRight size={28} /></button>}
    </div>
  )
}
