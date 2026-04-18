import React, { useState, useEffect } from 'react';
import { Box, Text, useInput } from 'ink';
import { Spinner } from '@inkjs/ui';
import { getTimeline } from '../csm.js';
import type { SessionMeta, TimelineEvent } from '../types.js';

interface Props {
  session: SessionMeta;
  onBack: () => void;
}

function fmtTime(iso: string): string {
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

export function TimelineView({ session, onBack }: Props) {
  const [events, setEvents] = useState<TimelineEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [scrollOffset, setScrollOffset] = useState(0);

  useEffect(() => {
    getTimeline(session.short_id)
      .then(data => { setEvents(data); setLoading(false); })
      .catch(() => setLoading(false));
  }, [session.short_id]);

  useInput((input, key) => {
    if (key.escape || input === 'q') { onBack(); return; }
    if (key.upArrow) setScrollOffset(o => Math.max(0, o - 1));
    if (key.downArrow) setScrollOffset(o => Math.min(Math.max(0, events.length - 20), o + 1));
  });

  const visibleEvents = events.slice(scrollOffset, scrollOffset + 25);

  return (
    <Box flexDirection="column" padding={1}>
      <Box marginBottom={1}>
        <Text bold>Timeline: </Text>
        <Text color="cyan">{session.title}</Text>
        <Text dimColor>  (esc to go back)</Text>
      </Box>
      {loading && <Spinner label="Loading timeline..." />}
      {visibleEvents.map((e, i) => {
        const color = e.type === 'user' ? 'green' : e.type === 'assistant' ? 'blue' : undefined;
        return (
          <Box key={i} gap={2}>
            <Text dimColor>{fmtTime(e.time)}</Text>
            <Text color={color}>{e.type.padEnd(14)}</Text>
            <Text wrap="truncate-end">
              {e.summary || (e.duration_ms ? `${(e.duration_ms / 1000).toFixed(1)}s` : '')}
            </Text>
          </Box>
        );
      })}
    </Box>
  );
}
