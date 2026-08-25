import { listWorkloads } from '../api/client'
import { usePolling } from '../hooks/usePolling'

export function WorkloadList() {
  const { data, error, loading } = usePolling(listWorkloads, 3000)

  if (loading && !data) return <p>Loading workloads…</p>
  if (error) return <p className="error">Failed to load workloads: {error}</p>

  const workloads = data?.workloads ?? []
  if (workloads.length === 0) {
    return <p className="empty">No workloads deployed yet — use the form above.</p>
  }

  return (
    <table>
      <thead>
        <tr>
          <th>Name</th>
          <th>Image</th>
          <th>Node</th>
          <th>State</th>
          <th>PID</th>
        </tr>
      </thead>
      <tbody>
        {workloads.map((w) => (
          <tr key={w.id}>
            <td>{w.name}</td>
            <td>{w.image}</td>
            <td>{w.node_name}</td>
            <td>
              <span className={`state state-${w.state}`}>{w.state}</span>
            </td>
            <td>{w.pid || '—'}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
