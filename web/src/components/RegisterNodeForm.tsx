import { useState } from 'react'
import { createJoinToken, BASE_URL } from '../api/client'

type Status =
  { kind: 'idle' } | { kind: 'success'; token: string } | { kind: 'error'; message: string }

// Registration itself always happens on the machine being added — an
// agent process calling the control plane with this token, not
// something a browser can do on that machine's behalf. This form's
// job is exactly what `ambudctl node generate-join-token` does: mint
// the token and hand over the command to run. See docs/MVP.md — every
// MVP capability, including node registration, is required to be
// clickable, not CLI-only.
export function RegisterNodeForm() {
  const [status, setStatus] = useState<Status>({ kind: 'idle' })
  const [submitting, setSubmitting] = useState(false)

  async function handleClick() {
    setSubmitting(true)
    try {
      const resp = await createJoinToken()
      setStatus({ kind: 'success', token: resp.token })
    } catch (err) {
      setStatus({ kind: 'error', message: err instanceof Error ? err.message : String(err) })
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="deploy-form">
      <button type="button" onClick={() => void handleClick()} disabled={submitting}>
        {submitting ? 'Generating…' : 'Generate join token'}
      </button>

      {status.kind === 'error' && <p className="error">{status.message}</p>}

      {status.kind === 'success' && (
        <div>
          <p>
            One-time token — it's only shown once. Run this on the machine you're adding (needs
            Linux + containerd; see docs/DEVELOPMENT.md):
          </p>
          <pre className="join-command">
            sudo ambud-agent --controlplane {BASE_URL} \{'\n'}
            {'  '}--join-token {status.token} \{'\n'}
            {'  '}--node-name {'<pick-a-name>'}
          </pre>
        </div>
      )}
    </div>
  )
}
