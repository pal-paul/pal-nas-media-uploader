import type { Album, MediaItem } from '../types/gallery'

export const mediaSeed: MediaItem[] = [
  { id: '1', title: 'Quiet shore', kind: 'photo', createdAt: '2026-08-28', tags: ['coast', 'summer'], url: 'https://images.unsplash.com/photo-1500530855697-b586d89ba3ee?auto=format&fit=crop&w=1200&q=85' },
  { id: '2', title: 'Late summer light', kind: 'photo', createdAt: '2026-08-27', tags: ['field', 'summer'], url: 'https://images.unsplash.com/photo-1470770841072-f978cf4d019e?auto=format&fit=crop&w=1200&q=85' },
  { id: '3', title: 'Mountain air', kind: 'video', createdAt: '2026-08-26', tags: ['mountain', 'trip'], url: 'https://images.unsplash.com/photo-1464822759023-fed622ff2c3b?auto=format&fit=crop&w=1200&q=85', duration: '0:18' },
  { id: '4', title: 'City in motion', kind: 'video', createdAt: '2026-08-19', tags: ['city', 'night'], url: 'https://images.unsplash.com/photo-1514565131-fce0801e5785?auto=format&fit=crop&w=1200&q=85', duration: '0:42' },
  { id: '5', title: 'Museum afternoon', kind: 'photo', createdAt: '2026-08-18', tags: ['city', 'art'], url: 'https://images.unsplash.com/photo-1561214115-f2f134cc4912?auto=format&fit=crop&w=1200&q=85' },
  { id: '6', title: 'Corner table', kind: 'photo', createdAt: '2026-08-17', tags: ['friends', 'food'], url: 'https://images.unsplash.com/photo-1528605248644-14dd04022da1?auto=format&fit=crop&w=1200&q=85' },
  { id: '7', title: 'Green passage', kind: 'photo', createdAt: '2026-08-10', tags: ['forest', 'walk'], url: 'https://images.unsplash.com/photo-1441974231531-c6227db76b6e?auto=format&fit=crop&w=1200&q=85' },
  { id: '8', title: 'Over the ridge', kind: 'photo', createdAt: '2026-08-09', tags: ['mountain', 'trip'], url: 'https://images.unsplash.com/photo-1500534623283-312aade485b7?auto=format&fit=crop&w=1200&q=85' },
  { id: '9', title: 'Sea glass', kind: 'video', createdAt: '2026-08-08', tags: ['coast', 'summer'], url: 'https://images.unsplash.com/photo-1498623116890-37e912163d5d?auto=format&fit=crop&w=1200&q=85', duration: '1:04' },
  { id: '10', title: 'Sunday market', kind: 'photo', createdAt: '2026-08-03', tags: ['market', 'city'], url: 'https://images.unsplash.com/photo-1488459716781-31db52582fe9?auto=format&fit=crop&w=1200&q=85' },
  { id: '11', title: 'Blue hour', kind: 'photo', createdAt: '2026-07-29', tags: ['city', 'night'], url: 'https://images.unsplash.com/photo-1519608487953-e999c86e7455?auto=format&fit=crop&w=1200&q=85' },
  { id: '12', title: 'Road north', kind: 'video', createdAt: '2026-07-27', tags: ['road', 'trip'], url: 'https://images.unsplash.com/photo-1500534314209-a25ddb2bd429?auto=format&fit=crop&w=1200&q=85', duration: '0:36' },
]

export const albumSeed: Album[] = [
  { id: '1', title: 'August 24-30', createdAt: '2026-08-30', automatic: true, itemCount: 3, mediaIds: ['1', '2', '3'] },
  { id: '2', title: 'August 17-23', createdAt: '2026-08-23', automatic: true, itemCount: 3, mediaIds: ['4', '5', '6'] },
  { id: '3', title: 'Mountain escape', createdAt: '2026-08-12', automatic: false, itemCount: 4, mediaIds: ['3', '7', '8', '12'] },
  { id: '4', title: 'Coastal days', createdAt: '2026-08-09', automatic: false, itemCount: 3, mediaIds: ['1', '2', '9'] },
  { id: '5', title: 'City notes', createdAt: '2026-08-04', automatic: false, itemCount: 4, mediaIds: ['4', '5', '10', '11'] },
]
