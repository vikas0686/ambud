import { useState, type FormEvent } from 'react'
import { createWorkload } from '../api/client'
import { deriveWorkloadName } from '../format'

type Status =
  { kind: 'idle' } | { kind: 'success'; message: string } | { kind: 'error'; message: string }

export function DeployForm() {
  const [image, setImage] = useState('')
  const [name, setName] = useState('')
  const [nodeID, setNodeID] = useState('')
  const [status, setStatus] = useState<Status>({ kind: 'idle' })
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setSubmitting(true)
    setStatus({ kind: 'idle' })

    const workloadName = name.trim() || deriveWorkloadName(image)
    try {
      const workload = await createWorkload({
        name: workloadName,
        image,
        node_id: nodeID.trim() || undefined,
      })
      setStatus({
        kind: 'success',
        message: `Deployed "${workload.name}" — it starts on the assigned node's next heartbeat.`,
      })
      setImage('')
      setName('')
      setNodeID('')
    } catch (err) {
      setStatus({ kind: 'error', message: err instanceof Error ? err.message : String(err) })
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={(e) => void handleSubmit(e)} className="deploy-form">
      <label>
        Image
        <input
          type="text"
          required
          placeholder="docker.io/library/nginx:alpine"
          value={image}
          onChange={(e) => setImage(e.target.value)}
        />
      </label>
      <label>
        Name <span className="hint">(optional — derived from image if omitted)</span>
        <input
          type="text"
          placeholder={image ? deriveWorkloadName(image) : ''}
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
      </label>
      <label>
        Node ID <span className="hint">(optional — auto-assigned if there's exactly one node)</span>
        <input type="text" value={nodeID} onChange={(e) => setNodeID(e.target.value)} />
      </label>
      <button type="submit" disabled={submitting || !image}>
        {submitting ? 'Deploying…' : 'Deploy'}
      </button>

      {status.kind === 'success' && <p className="success">{status.message}</p>}
      {status.kind === 'error' && <p className="error">{status.message}</p>}
    </form>
  )
}
