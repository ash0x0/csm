import React from 'react';
import { Box, Text } from 'ink';
import { Spinner } from '@inkjs/ui';
import type { SessionMeta, TimelineEvent } from '../types.js';

interface Props {
  session: SessionMeta | null;
  events: TimelineEvent[];
  loading: boolean;
  error: string | null;
  width: number;
  height: number;
}

function fmtDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  return `${Math.floor(ms / 60000)}m ${Math.floor((ms % 60000) / 1000)}s`;
}

export function PreviewPanel({ session, events, loading, error, height }: Props) {
  if (!session) {
    return (
      <Box flexDirection="column" paddingX={1}>
        <Text dimColor>No session selected</Text>
      </Box>
    );
  }

  const userEvents = events.filter(e => e.type === 'user');
  const lastUserEvent = userEvents[userEvents.length - 1];
  const turnDurations = events.filter(e => e.type === 'turn-duration');
  const avgMs = turnDurations.length
    ? turnDurations.reduce((s, e) => s + (e.duration_ms ?? 0), 0) / turnDurations.length
    : null;

  const branch = session.branch && session.branch !== 'HEAD' ? session.branch : '—';
  const project = session.project.split('/').pop() ?? session.project;

  return (
    <Box flexDirection="column" paddingX={1}>
      <Text bold>{session.title.length > 40 ? session.title.slice(0, 37) + '...' : session.title}</Text>
      <Text dimColor>{session.short_id}</Text>
      <Box marginTop={1} flexDirection="column">
        <Text>Project: <Text color="cyan">{project}</Text></Text>
        <Text>Branch:  <Text color="yellow">{branch}</Text></Text>
        <Text>Msgs:    <Text color="green">{session.messages}</Text></Text>
        {avgMs !== null && <Text>Avg turn: {fmtDuration(avgMs)}</Text>}
        {session.is_active && <Text color="green" bold>● ACTIVE</Text>}
      </Box>

      {loading && (
        <Box marginTop={1}>
          <Spinner label="Loading preview..." />
        </Box>
      )}

      {error && (
        <Box marginTop={1}>
          <Text color="red">Error: {error}</Text>
        </Box>
      )}

      {!loading && !error && lastUserEvent && (
        <Box marginTop={1} flexDirection="column">
          <Text dimColor bold>Last prompt:</Text>
          <Text wrap="wrap">
            {lastUserEvent.summary.slice(0, Math.max(50, (height - 12) * 30))}
          </Text>
        </Box>
      )}
    </Box>
  );
}
