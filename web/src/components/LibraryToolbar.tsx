import { CalendarDays, CalendarRange, ChevronDown, Search, SlidersHorizontal, X } from 'lucide-react'
import type { LibraryView, MediaFilter, MediaGrouping, MediaSort } from '../types/gallery'

type Props = {
  view: LibraryView
  query: string
  filter: MediaFilter
  sort: MediaSort
  grouping: MediaGrouping
  onQueryChange: (value: string) => void
  onFilterChange: (value: MediaFilter) => void
  onSortChange: (value: MediaSort) => void
  onGroupingChange: (value: MediaGrouping) => void
}

export function LibraryToolbar({ view, query, filter, sort, grouping, onQueryChange, onFilterChange, onSortChange, onGroupingChange }: Props) {
  return (
    <section className="toolbar" aria-label="Library controls">
      <label className="search-box">
        <Search size={18} />
        <input value={query} onChange={(event) => onQueryChange(event.target.value)} placeholder="Search names, camera, tags, dates, or dimensions" />
        {query && <button onClick={() => onQueryChange('')} title="Clear search"><X size={16} /></button>}
      </label>
      {!['albums', 'map', 'storage'].includes(view) && (
        <div className="filter-group">
          <label className="select-control"><SlidersHorizontal size={16} /><select value={filter} onChange={(event) => onFilterChange(event.target.value as MediaFilter)}><option value="all">All types</option><option value="photo">Photos</option><option value="video">Videos</option></select><ChevronDown size={15} /></label>
          <label className="select-control"><CalendarDays size={16} /><select value={sort} onChange={(event) => onSortChange(event.target.value as MediaSort)}><option value="newest">Newest first</option><option value="oldest">Oldest first</option><option value="title">Title A-Z</option></select><ChevronDown size={15} /></label>
          <label className="select-control"><CalendarRange size={16} /><select value={grouping} onChange={(event) => onGroupingChange(event.target.value as MediaGrouping)}><option value="none">Grid</option><option value="day">By day</option><option value="month">By month</option><option value="year">By year</option></select><ChevronDown size={15} /></label>
        </div>
      )}
    </section>
  )
}
