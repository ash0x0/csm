import { useState, useEffect, useRef } from 'react';
import { getTimeline } from '../csm.js';
import type { SessionMeta, TimelineEvent } from '../types.js';

interface PreviewData {
  events: TimelineEvent[];
  loading: boolean;
  error: string | null;
}

export function usePreview(session: SessionMeta | null): PreviewData {
  const [events, setEvents] = useState<TimelineEvent[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (!session) {
      setEvents([]);
      return;
    }

    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(async () => {
      setLoading(true);
      setError(null);
      try {
        const data = await getTimeline(session.short_id);
        setEvents(data);
      } catch (e) {
        setError(String(e));
        setEvents([]);
      } finally {
        setLoading(false);
      }
    }, 150);

    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, [session?.id]);

  return { events, loading, error };
}
