import { useState, useEffect, useCallback } from 'react';
import { listSessions, searchSessions } from '../csm.js';
import type { SessionMeta } from '../types.js';

interface SessionsState {
  sessions: SessionMeta[];
  loading: boolean;
  error: string | null;
  refresh: () => void;
}

export function useSessions(query: string): SessionsState {
  const [sessions, setSessions] = useState<SessionMeta[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);

  const refresh = useCallback(() => setTick(t => t + 1), []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);

    const fetch = query.trim()
      ? searchSessions(query.trim()).then(results => results.map(r => r.session))
      : listSessions();

    fetch
      .then(data => { if (!cancelled) { setSessions(data); setLoading(false); } })
      .catch(e => { if (!cancelled) { setError(String(e)); setLoading(false); } });

    return () => { cancelled = true; };
  }, [query, tick]);

  return { sessions, loading, error, refresh };
}
