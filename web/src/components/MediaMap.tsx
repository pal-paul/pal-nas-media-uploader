import { CircleMarker, MapContainer, Popup, TileLayer } from 'react-leaflet'
import 'leaflet/dist/leaflet.css'
import type { MediaItem } from '../types/gallery'
import { AuthenticatedImage } from './AuthenticatedMedia'

type Props = { media: MediaItem[]; onView: (id: string) => void }

export function MediaMap({ media, onView }: Props) {
  const located = media.filter((item) => item.latitude != null && item.longitude != null)
  if (!located.length) return <section className="empty-state"><h2>No mapped memories</h2><p>Photos with GPS metadata will appear here after the next scan.</p></section>
  const center: [number, number] = [located[0].latitude!, located[0].longitude!]
  return (
    <section className="media-map" aria-label="Media map">
      <MapContainer center={center} zoom={6} scrollWheelZoom>
        <TileLayer attribution="&copy; OpenStreetMap contributors" url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png" />
        {located.map((item) => (
          <CircleMarker key={item.id} center={[item.latitude!, item.longitude!]} radius={8} pathOptions={{ color: '#fff', weight: 2, fillColor: '#176fd1', fillOpacity: .9 }}>
            <Popup>
              <button className="map-preview" onClick={() => onView(item.id)}>
                <AuthenticatedImage src={item.thumbnailUrl ?? item.url} alt={item.title} />
                <strong>{item.title}</strong>
                <span>{item.camera || item.fileName}</span>
              </button>
            </Popup>
          </CircleMarker>
        ))}
      </MapContainer>
    </section>
  )
}
