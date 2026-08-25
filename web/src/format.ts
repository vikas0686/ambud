// Small formatting helpers shared by the node/workload list views. No
// date/number library — the formatting needs here are simple enough
// that one would be premature (see web/README.md).

export function formatBytes(bytes: number | undefined): string {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const exp = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / 1024 ** exp).toFixed(1)} ${units[exp]}`
}

export function formatHeartbeat(lastHeartbeatAt: string | null | undefined): string {
  if (!lastHeartbeatAt) return 'never'
  const seconds = Math.max(0, Math.round((Date.now() - new Date(lastHeartbeatAt).getTime()) / 1000))
  if (seconds < 60) return `${seconds}s ago`
  return `${Math.round(seconds / 60)}m ago`
}

// Mirrors cmd/ambudctl/run.go's defaultContainerName — the CLI and the
// UI should derive the same default name from the same image
// reference, e.g. "docker.io/library/nginx:alpine" -> "nginx".
export function deriveWorkloadName(image: string): string {
  let ref = image
  const at = ref.indexOf('@')
  if (at !== -1) ref = ref.slice(0, at)
  const slash = ref.lastIndexOf('/')
  if (slash !== -1) ref = ref.slice(slash + 1)
  const colon = ref.lastIndexOf(':')
  if (colon !== -1) ref = ref.slice(0, colon)
  return ref
}
