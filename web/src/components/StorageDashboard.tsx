import { Database, Film, Images, Layers3 } from 'lucide-react'
import type { StorageStats } from '../types/gallery'

const formatBytes = (bytes: number) => {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / 1024 ** index).toFixed(index > 1 ? 1 : 0)} ${units[index]}`
}

export function StorageDashboard({ stats }: { stats: StorageStats }) {
  const metrics = [
    { label: 'All media', value: formatBytes(stats.totalBytes), detail: `${stats.photoCount + stats.videoCount} items`, icon: Database },
    { label: 'Photos', value: formatBytes(stats.photoBytes), detail: `${stats.photoCount} photos`, icon: Images },
    { label: 'Videos', value: formatBytes(stats.videoBytes), detail: `${stats.videoCount} videos`, icon: Film },
    { label: 'Thumbnail cache', value: formatBytes(stats.thumbnailCacheBytes), detail: 'Regenerates automatically', icon: Layers3 },
  ]
  return (
    <section className="storage-dashboard">
      <div className="storage-metrics">{metrics.map(({ label, value, detail, icon: Icon }) => <article key={label}><Icon size={20} /><span>{label}</span><strong>{value}</strong><small>{detail}</small></article>)}</div>
      <div className="storage-lists">
        <section><h2>Largest videos</h2>{stats.largeVideos.length ? <ol>{stats.largeVideos.map((item) => <li key={item.id}><span>{item.fileName || item.title}</span><strong>{formatBytes(item.fileSize ?? 0)}</strong></li>)}</ol> : <p>No video size metadata yet.</p>}</section>
        <section><h2>Possible duplicates</h2><p className="storage-note">Candidates share the same file size. Review before deleting.</p>{stats.duplicateGroups.length ? <ol>{stats.duplicateGroups.map((group) => <li key={group.fileSize}><span>{group.items.map((item) => item.fileName || item.title).join(', ')}</span><strong>{formatBytes(group.fileSize)}</strong></li>)}</ol> : <p>No duplicate candidates found.</p>}</section>
      </div>
    </section>
  )
}
