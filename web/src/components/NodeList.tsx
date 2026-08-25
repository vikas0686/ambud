import { listNodes } from '../api/client'
import { usePolling } from '../hooks/usePolling'
import { formatBytes, formatHeartbeat } from '../format'

export function NodeList() {
  const { data, error, loading } = usePolling(listNodes, 3000)

  if (loading && !data) return <p>Loading nodes…</p>
  if (error) return <p className="error">Failed to load nodes: {error}</p>

  const nodes = data?.nodes ?? []
  if (nodes.length === 0) {
    return (
      <p className="empty">
        No nodes registered yet. Run <code>ambudctl node generate-join-token</code>, then start an
        agent with it.
      </p>
    )
  }

  return (
    <table>
      <thead>
        <tr>
          <th>Name</th>
          <th>Status</th>
          <th>Last heartbeat</th>
          <th>CPU</th>
          <th>Memory</th>
        </tr>
      </thead>
      <tbody>
        {nodes.map((n) => (
          <tr key={n.id}>
            <td>{n.name}</td>
            <td>
              <span className={`state state-${n.status}`}>{n.status}</span>
            </td>
            <td>{formatHeartbeat(n.last_heartbeat_at)}</td>
            <td>
              {n.resources?.cpu_cores ?? 0} cores, {(n.resources?.cpu_used_percent ?? 0).toFixed(1)}
              %
            </td>
            <td>
              {formatBytes(n.resources?.mem_used_bytes)} /{' '}
              {formatBytes(n.resources?.mem_total_bytes)}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
