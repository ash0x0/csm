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

export function PreviewPanel({ session, events, loading, error, width, height }: Props) {
  if (!session) {
    return (
      <Box flexDirection="column" paddingX={1}>
        <Text dimColor>No session selected</Text>
      </Box>
    );
  }

  const userEvents = events.filter(e => e.type === 'user');
  const recentPrompts = userEvents.slice(-3).reverse();
  const turnDurations = events.filter(e => e.type === 'turn-duration');
  const avgMs = turnDurations.length
    ? turnDurations.reduce((s, e) => s + (e.duration_ms ?? 0), 0) / turnDurations.length
    : null;

  const branch = session.branch && session.branch !== 'HEAD' ? session.branch : '—';
  const project = session.project.split('/').pop() ?? session.project;
  const truncWidth = Math.max(30, width - 4);
  const promptLineWidth = Math.max(40, truncWidth);

  const promptLabels = ['Last prompt:', 'Prev:', 'Prev 2:'];

  return (
    <Box flexDirection="column" paddingX={1}>
      <Text bold color="cyanBright">
        {session.title.length > 40 ? session.title.slice(0, 37) + '...' : session.title}
      </Text>
      <Text color="blue">{session.short_id}</Text>
      <Box marginTop={1} flexDirection="column">
        <Text><Text color="magenta">Project:</Text> <Text color="cyan">{project}</Text></Text>
        <Text><Text color="magenta">Branch: </Text> <Text color="yellow">{branch}</Text></Text>
        <Text><Text color="magenta">Msgs:   </Text> <Text color="green">{session.messages}</Text></Text>
        {avgMs !== null && <Text><Text color="magenta">Avg turn:</Text> {fmtDuration(avgMs)}</Text>}
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

      {!loading && !error && recentPrompts.length > 0 && (
        <Box marginTop={1} flexDirection="column">
          <Text color="blue">{'─'.repeat(Math.min(truncWidth, 30))}</Text>
          {recentPrompts.map((evt, idx) => (
            <Box key={idx} flexDirection="column" marginTop={idx > 0 ? 1 : 0}>
              <Text color="cyan" bold>{promptLabels[idx]}</Text>
              <Text wrap="wrap">
                {evt.summary.slice(0, promptLineWidth * Math.max(2, Math.floor((height - 14) / recentPrompts.length)))}
              </Text>
            </Box>
          ))}
        </Box>
      )}
    </Box>
  );
}
