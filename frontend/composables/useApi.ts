import { defineStore } from 'pinia'

export const useApiStore = defineStore('api', {
  state: () => ({
    tenantId: '',
    apiBase: '/api',
  }),
  getters: {
    headers: (state) => ({
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${state.tenantId}`,
    }),
  },
  actions: {
    setTenant(id: string) {
      this.tenantId = id
    },
    async get(path: string) {
      const res = await fetch(`${this.apiBase}${path}`, { headers: this.headers })
      if (!res.ok) throw new Error(`API Error: ${res.status}`)
      return res.json()
    },
    async post(path: string, body: any) {
      const res = await fetch(`${this.apiBase}${path}`, {
        method: 'POST',
        headers: this.headers,
        body: JSON.stringify(body),
      })
      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || `API Error: ${res.status}`)
      }
      return res.json()
    },
    async put(path: string, body: any) {
      const res = await fetch(`${this.apiBase}${path}`, {
        method: 'PUT',
        headers: this.headers,
        body: JSON.stringify(body),
      })
      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || `API Error: ${res.status}`)
      }
      return res.json()
    },
    async del(path: string) {
      const res = await fetch(`${this.apiBase}${path}`, {
        method: 'DELETE',
        headers: this.headers,
      })
      if (!res.ok) throw new Error(`API Error: ${res.status}`)
      return res.json()
    },
  },
})
