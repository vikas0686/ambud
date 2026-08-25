import type { components } from './schema'

// Plain fetch wrappers typed against the generated schema (see
// generate:api in package.json) — no client library on top. There are
// only four calls today; a generated client (openapi-fetch or similar)
// is worth adding once there's enough surface for hand-written
// boilerplate to actually hurt, not before. See web/README.md.

export type NodeStatus = components['schemas']['NodeStatus']
export type WorkloadStatus = components['schemas']['WorkloadStatus']
export type CreateWorkloadRequest = components['schemas']['CreateWorkloadRequest']
export type CreateJoinTokenResponse = components['schemas']['CreateJoinTokenResponse']
export type ErrorResponse = components['schemas']['ErrorResponse']

// Same default as ambud-controlplane's --listen; override via
// VITE_CONTROLPLANE_URL for a dev server pointed elsewhere. Exported so
// components can show it in copy-pasteable instructions (see
// RegisterNodeForm) without duplicating the same default.
export const BASE_URL = import.meta.env.VITE_CONTROLPLANE_URL ?? 'http://localhost:8081'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...init?.headers },
  })

  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as ErrorResponse | null
    throw new Error(body?.error ?? res.statusText)
  }

  return res.json() as Promise<T>
}

export function listNodes(): Promise<{ nodes: NodeStatus[] }> {
  return request('/v1/nodes')
}

export function listWorkloads(): Promise<{ workloads: WorkloadStatus[] }> {
  return request('/v1/workloads')
}

export function createWorkload(req: CreateWorkloadRequest): Promise<WorkloadStatus> {
  return request('/v1/workloads', { method: 'POST', body: JSON.stringify(req) })
}

export function createJoinToken(): Promise<CreateJoinTokenResponse> {
  return request('/v1/join-tokens', { method: 'POST' })
}
