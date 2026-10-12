import { useSyncExternalStore } from 'react'
import { getState, subscribe } from './store'

export function useAuth() {
  return useSyncExternalStore(subscribe, getState)
}
